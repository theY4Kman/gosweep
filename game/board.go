package game

import (
	"github.com/faiface/pixel"
	"github.com/gammazero/deque"
	"github.com/they4kman/gosweep/util/lockedRand"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type boardConfig struct {
	Width, Height uint
	NumMines      uint
	Mode          GameMode

	Seed int64

	Director Director

	OnGameEnd func(*Board)
}

type Board struct {
	width, height uint // in number of cells
	numMines      uint
	mode          GameMode

	initialSeed int64
	rand        *rand.Rand

	state          BoardState
	cells          [][]Cell
	hasClicked     bool
	numFlags       uint
	remainingCells map[*Cell]struct{}

	revelations chan *Cell
	actionGroup sync.WaitGroup

	director             Director
	directorFrame        int64
	directorAct          chan struct{}
	directorPause        chan struct{}
	directorStop         chan struct{}
	directorActRequested *sync.Cond
	directorCellChanges  chan *Cell

	directorAnnotations deque.Deque

	onGameEnd func(*Board)
}

func (board *Board) Width() uint {
	return board.width
}

func (board *Board) Height() uint {
	return board.height
}

func (board *Board) NumCells() uint {
	return board.width * board.height
}

func (board *Board) NumMinesRemaining() uint {
	return board.numMines - board.numFlags
}

func (board *Board) Rand() *rand.Rand {
	return board.rand
}

func (board *Board) CellAt(x, y uint) *Cell {
	if x >= 0 && y >= 0 && x < board.width && y < board.height {
		return &board.cells[y][x]
	}
	return nil
}

func (board *Board) Cells() <-chan *Cell {
	out := make(chan *Cell)
	go func() {
		for y := uint(0); y < board.height; y++ {
			for x := uint(0); x < board.width; x++ {
				out <- board.CellAt(x, y)
			}
		}
		close(out)
	}()
	return out
}

func (board *Board) UnrevealedCells() <-chan *Cell {
	out := make(chan *Cell)
	go func() {
		for cell := range board.Cells() {
			if !cell.isRevealed {
				out <- cell
			}
		}
		close(out)
	}()
	return out
}

func (board *Board) TogglePaused() {
	if board.state == Ongoing {
		board.state = Paused
	} else if board.state == Paused {
		board.state = Ongoing
	} else {
		return
	}
	board.directorPause <- struct{}{}
}

func (board *Board) RequestDirectorAct() {
	if board.directorActRequested != nil {
		board.directorActRequested.Broadcast()
	}
}

func (board *Board) AddAnnotation(annotation Annotation) {
	annotations := make(chan Annotation, 1)
	annotations <- annotation
	close(annotations)
	board.AddAnnotations(annotations)
}

func (board *Board) AddAnnotations(annotations <-chan Annotation) {
	for annotation := range annotations {
		annotation.frame = board.directorFrame
		annotation.firstShown = time.Now()
		board.directorAnnotations.PushBack(annotation)
	}
}

func (board *Board) serialize() string {
	builder := strings.Builder{}
	builder.Grow(int(board.height*board.width + board.height))

	lastY := board.height - 1
	for y := uint(0); y < board.height; y++ {
		for x := uint(0); x < board.width; x++ {
			cell := board.CellAt(x, y)
			builder.WriteString(cell.serialize())
		}

		if y != lastY {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func (board *Board) snapshot() *BoardSnapshot {
	return &BoardSnapshot{
		Seed:            board.initialSeed,
		SerializedBoard: board.serialize(),
	}
}

func (board *Board) screenToGridCoords(pos pixel.Vec) (uint, uint) {
	return uint(pos.X) / cellWidth, board.height - uint(pos.Y)/cellWidth - 1
}

func (board *Board) canPlay() bool {
	return board.state == Ongoing || board.state == Paused
}

func (board *Board) win() {
	board.state = Won
	board.endGame()
}

func (board *Board) lose() {
	board.state = Lost
	board.endGame()

	wg := sync.WaitGroup{}

	revealLost := func(cells <-chan *Cell) {
		for cell := range cells {
			cell.revealLost()
		}
		wg.Done()
	}

	cells := board.Cells()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go revealLost(cells)
	}

	wg.Wait()
}

func (board *Board) endGame() {
	if board.director != nil {
		board.directorStop <- struct{}{}
		close(board.directorStop)
		board.directorStop = nil

		close(board.directorPause)
		board.directorPause = nil

		board.director.End()
	}

	if board.onGameEnd != nil {
		board.onGameEnd(board)
	}
}

func (board *Board) startGame() {
	if board.director != nil {
		board.director.Init(board)
		board.director.ActContinuously(board.directorAct, board.directorStop)

		// Emit all cells to director at start of game
		initialCells := make(chan *Cell, board.NumCells())
		for cell := range board.Cells() {
			initialCells <- cell
		}
		close(initialCells)
		board.director.CellChanges(initialCells)
	}
}

func (board *Board) markRevealed(cell *Cell) {
	board.revelations <- cell
}

func (board *Board) markChanged(cell *Cell) {
	if board.directorCellChanges != nil {
		board.directorCellChanges <- cell
	}
}

func (board *Board) clearSurroundingMines(center *Cell) {
	wg := sync.WaitGroup{}

	possibleRelocationsMap := make(map[*Cell]struct{})
	for cell := range board.Cells() {
		if !cell.isMine {
			possibleRelocationsMap[cell] = struct{}{}
		}
	}

	decreaseNumMines := make(chan *Cell)
	wg.Add(1)
	go func() {
		for cell := range decreaseNumMines {
			atomic.AddUint32(&cell.numMines, ^uint32(0))
		}
		wg.Done()
	}()

	numSurroundingMines := 0
	for cell := range center.SelfNeighbors() {
		delete(possibleRelocationsMap, cell)

		if cell.isMine {
			numSurroundingMines++

			cell.isMine = false
			board.remainingCells[cell] = struct{}{}

			cell.SendNeighbors(decreaseNumMines)
		}
	}
	close(decreaseNumMines)

	possibleRelocations := make([]*Cell, len(possibleRelocationsMap))

	i := 0
	for cell := range possibleRelocationsMap {
		possibleRelocations[i] = cell
		i++
	}

	board.rand.Shuffle(len(possibleRelocations), func(i, j int) {
		possibleRelocations[i], possibleRelocations[j] = possibleRelocations[j], possibleRelocations[i]
	})

	increaseNumMines := make(chan *Cell)
	wg.Add(1)
	go func() {
		for cell := range increaseNumMines {
			atomic.AddUint32(&cell.numMines, 1)
		}
		wg.Done()
	}()

	for i := 0; i < numSurroundingMines; i++ {
		cell := possibleRelocations[i]
		cell.isMine = true
		delete(board.remainingCells, cell)

		cell.SendNeighbors(increaseNumMines)
	}
	close(increaseNumMines)

	wg.Wait()
}

func (board *Board) fillMines(cells <-chan *Cell) {
	mineNeighbors := make(chan *Cell)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for cell := range mineNeighbors {
			atomic.AddUint32(&cell.numMines, 1)

			if cell.isRevealed {
				cell.setState(CellState(cell.numMines))
			}
		}
		wg.Done()
	}()

	for cell := range cells {
		cell.isMine = true
		board.numMines++
		delete(board.remainingCells, cell)

		cell.SendNeighbors(mineNeighbors)
	}
	close(mineNeighbors)

	wg.Wait()
}

func (board *Board) randomCells(n uint) <-chan *Cell {
	cells := make(chan *Cell, n)

	numCells := board.height * board.width
	cellIndexes := make([]uint, numCells)
	for cellIdx := uint(0); cellIdx < numCells; cellIdx++ {
		cellIndexes[cellIdx] = cellIdx
	}

	board.rand.Shuffle(len(cellIndexes), func(i, j int) {
		cellIndexes[i], cellIndexes[j] = cellIndexes[j], cellIndexes[i]
	})
	for i := uint(0); i < n; i++ {
		cellIdx := cellIndexes[i]
		y, x := cellIdx/board.width, cellIdx%board.width
		cells <- board.CellAt(x, y)
	}

	close(cells)
	return cells
}

func createBoard(config boardConfig) *Board {
	board := Board{
		width:    config.Width,
		height:   config.Height,
		numMines: 0, // this will be set to its final value by fillMines
		mode:     config.Mode,

		initialSeed: config.Seed,
		rand:        lockedRand.NewFromSeed(config.Seed),

		state:          Ongoing,
		cells:          make([][]Cell, config.Height),
		hasClicked:     false,
		numFlags:       0,
		remainingCells: make(map[*Cell]struct{}),

		actionGroup: sync.WaitGroup{},
		revelations: make(chan *Cell),

		director: config.Director,

		onGameEnd: config.OnGameEnd,
	}

	if config.Director != nil {
		board.directorAct = make(chan struct{})
		board.directorPause = make(chan struct{})
		board.directorStop = make(chan struct{}, 1)
		board.directorActRequested = sync.NewCond(&sync.Mutex{})
		board.directorCellChanges = make(chan *Cell, board.NumCells())

		go func() {
			// Allow the game to start paused
			select {
			case <-board.directorPause:
				<-board.directorPause
			default:
			}

			for {
				select {
				case <-board.directorAct:
					board.RequestDirectorAct()
				case <-board.directorPause:
					<-board.directorPause
				}
			}
		}()

		go func() {
			for {
				board.directorActRequested.L.Lock()
				board.directorActRequested.Wait()

				if board.directorStop == nil {
					return
				}

				cellChanges := board.directorCellChanges
				board.directorCellChanges = make(chan *Cell, board.NumCells())

				close(cellChanges)
				board.director.CellChanges(cellChanges)

				actions := make(chan CellAction, board.NumCells())
				board.directorFrame++
				go board.director.Act(actions)

				dedupedActions := make(map[CellAction]struct{})
				for cellAction := range actions {
					dedupedActions[cellAction] = struct{}{}
				}

				for cellAction := range dedupedActions {
					annotation := Annotation{
						Type:       AnnotationType(cellAction.action),
						Cell:       cellAction.cell,
						frame:      board.directorFrame,
						firstShown: time.Now(),
					}
					board.directorAnnotations.PushBack(annotation)

					cellAction.perform()
				}

				board.directorActRequested.L.Unlock()
			}
		}()
	}

	// Perform all unrevealedCell modifications in a single goroutine, to avoid
	// concurrent modifications
	go func() {
		for cell := range board.revelations {
			delete(board.remainingCells, cell)

			if len(board.remainingCells) == 0 {
				board.win()
			}
		}
	}()

	cellIdx := uint(0)
	for y := uint(0); y < config.Height; y++ {
		row := make([]Cell, config.Width)
		board.cells[y] = row

		for x := uint(0); x < config.Width; x++ {
			cell := &board.cells[y][x]
			cell.board = &board
			cell.idx = cellIdx
			cell.x, cell.y = x, y
			cell.isMine = false
			cell.isLosingMine = false
			cell.isFlagged = false
			cell.isRevealed = false
			cell.numMines = 0
			cell.state = Unrevealed
			cell.sprite = cellSprites[Unrevealed]
			cell.isDirty = true

			board.remainingCells[cell] = struct{}{}

			cellIdx++
		}
	}

	return &board
}

func createFilledBoard(config boardConfig) *Board {
	board := createBoard(config)
	board.fillMines(board.randomCells(config.NumMines))
	return board
}
