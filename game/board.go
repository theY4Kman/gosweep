package game

import (
	"github.com/faiface/pixel"
	"math/rand"
	"sync/atomic"
)

type Board struct {
	width, height uint // in number of cells
	numMines      uint
	cells         [][]Cell

	state          BoardState
	numFlags       uint
	remainingCells map[*Cell]struct{}

	revelations chan *Cell

	director Director
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

func (board *Board) screenToGridCoords(pos pixel.Vec) (uint, uint) {
	return uint(pos.X) / cellWidth, board.height - uint(pos.Y)/cellWidth - 1
}

func (board *Board) canPlay() bool {
	return board.state == Ongoing
}

func (board *Board) win() {
	board.state = Won
	board.endGame()
}

func (board *Board) lose() {
	board.state = Lost
	board.endGame()

	revealLost := func(cells <-chan *Cell) {
		for cell := range cells {
			cell.revealLost()
		}
	}

	cells := board.Cells()

	for i := 0; i < 4; i++ {
		go revealLost(cells)
	}
}

func (board *Board) endGame() {
	if board.director != nil {
		board.director.End()
	}
}

func (board *Board) startGame() {
	if board.director != nil {
		board.director.Init(board)
		board.director.ActContinuously()
	}
}

func (board *Board) markRevealed(cell *Cell) {
	board.revelations <- cell
}

func (board *Board) clearSurroundingMines(center *Cell) {
	possibleRelocationsMap := make(map[*Cell]struct{})
	for cell := range board.Cells() {
		if !cell.isMine {
			possibleRelocationsMap[cell] = struct{}{}
		}
	}

	decreaseNumMines := make(chan *Cell)
	go func() {
		for cell := range decreaseNumMines {
			atomic.AddUint32(&cell.numMines, ^uint32(1-1))
		}
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

	rand.Shuffle(len(possibleRelocations), func(i, j int) {
		possibleRelocations[i], possibleRelocations[j] = possibleRelocations[j], possibleRelocations[i]
	})

	increaseNumMines := make(chan *Cell)
	go func() {
		for cell := range increaseNumMines {
			atomic.AddUint32(&cell.numMines, 1)
		}
	}()

	for i := 0; i<numSurroundingMines; i++ {
		cell := possibleRelocations[i]
		cell.isMine = true
		delete(board.remainingCells, cell)

		cell.SendNeighbors(increaseNumMines)
	}
	close(increaseNumMines)
}

func createBoard(width uint, height uint, numMines uint, director Director) *Board {
	board := Board{
		state:          Ongoing,
		width:          width,
		height:         height,
		numMines:       numMines,
		cells:          make([][]Cell, height),
		remainingCells: make(map[*Cell]struct{}),
		revelations:    make(chan *Cell),
		director:       director,
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

	// Store cell indexes, to shuffle later and fill mines
	cellIndexes := make([]uint, height*width)
	cellIdx := uint(0)

	for y := uint(0); y < height; y++ {
		row := make([]Cell, width)
		board.cells[y] = row

		for x := uint(0); x < width; x++ {
			cell := &board.cells[y][x]
			cell.board = &board
			cell.idx = cellIdx
			cell.x, cell.y = x, y
			cell.numMines = 0
			cell.state = Unrevealed
			cell.sprite = cellSprites[Unrevealed]
			cell.isDirty = true

			board.remainingCells[cell] = struct{}{}

			cellIndexes[cellIdx] = cellIdx
			cellIdx++
		}
	}

	mineNeighborChan := make(chan *Cell, numMines)

	go func() {
		for cell := range mineNeighborChan {
			atomic.AddUint32(&cell.numMines, 1)
		}
	}()

	rand.Shuffle(len(cellIndexes), func(i, j int) {
		cellIndexes[i], cellIndexes[j] = cellIndexes[j], cellIndexes[i]
	})
	for i := uint(0); i < numMines; i++ {
		cellIdx = cellIndexes[i]
		y, x := cellIdx/width, cellIdx%width
		cell := board.CellAt(x, y)
		cell.isMine = true
		delete(board.remainingCells, cell)

		cell.SendNeighbors(mineNeighborChan)
	}
	close(mineNeighborChan)

	return &board
}
