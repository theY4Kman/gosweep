package constraint

import (
	"github.com/they4kman/gosweep/director/random"
	"github.com/they4kman/gosweep/game"
	"sync"
)

type Director struct {
	game.BaseDirector

	board *game.Board

	act      chan chan<- game.CellAction
	hasActed bool

	observations       map[*Observation]struct{}
	observationsByCell map[*game.Cell]map[*Observation]struct{}
	observationsLock   sync.Mutex
}

type Observation struct {
	origin   *game.Cell
	numMines int
	cells    map[*game.Cell]struct{}
}

func (director *Director) Init(board *game.Board) {
	director.board = board
	director.act = make(chan chan<- game.CellAction)
	director.hasActed = false

	director.observations = make(map[*Observation]struct{})
	director.observationsByCell = make(map[*game.Cell]map[*Observation]struct{})

	director.observationsLock = sync.Mutex{}

	go func() {
		for actions := range director.act {
			if !director.hasActed {
				director.actRandom(actions)
				director.hasActed = true
			} else {
				director.actDeliberate(actions)
			}
		}
	}()
}

func (director *Director) actRandom(actions chan<- game.CellAction) {
	randomDirector := &random.Director{}
	randomDirector.Init(director.board)
	randomDirector.Act(actions)
	randomDirector.End()
}

func (director *Director) actDeliberate(actions chan<- game.CellAction) {
	director.observationsLock.Lock()
	defer director.observationsLock.Unlock()

	for observation := range director.observations {
		if observation.numMines == len(observation.cells) {
			for cell := range observation.cells {
				actions <- cell.RightClick()
			}

			observation.cells = nil
			observation.numMines = 0
			delete(director.observations, observation)

		} else if observation.numMines == 0 {
			for cell := range observation.cells {
				actions <- cell.Click()
			}

			observation.cells = nil
			observation.numMines = 0
			delete(director.observations, observation)
		}
	}

	close(actions)
}

func (director *Director) Act(actions chan<- game.CellAction) {
	director.act <- actions
}

func (director *Director) CellChanges(changes <-chan *game.Cell) {
	for cell := range changes {
		if cell.IsRevealed() {
			director.CellRevealed(cell)
		}

		if cellObservations, ok := director.observationsByCell[cell]; ok {
			for observation := range cellObservations {
				if observation.numMines == 0 || len(observation.cells) == 0 {
					delete(cellObservations, observation)
				} else if cell.IsFlagged() {
					delete(observation.cells, cell)
					observation.numMines--
				} else if cell.IsRevealed() {
					delete(observation.cells, cell)
				}
			}
		}
	}
}

func (director *Director) CellRevealed(cell *game.Cell) {
	observation := Observation{
		origin:   cell,
		numMines: int(cell.NumMines()),
		cells:    make(map[*game.Cell]struct{}),
	}

	for neighbor := range cell.Neighbors() {
		if !neighbor.IsRevealed() {
			if neighbor.IsFlagged() {
				observation.numMines--
			} else {
				observation.cells[neighbor] = struct{}{}
			}
		}
	}

	if len(observation.cells) == 0 {
		return
	}

	director.observationsLock.Lock()
	defer director.observationsLock.Unlock()

	director.observations[&observation] = struct{}{}
	for cell := range observation.cells {
		var cellObservations map[*Observation]struct{}
		var exists bool
		if cellObservations, exists = director.observationsByCell[cell]; !exists {
			cellObservations = make(map[*Observation]struct{})
			director.observationsByCell[cell] = cellObservations
		}

		cellObservations[&observation] = struct{}{}
	}

	//XXX///////////////////////////////////////////////////////////////////////////////////////////
	//fmt.Fprintf(os.Stdout,
	//	"Found observation, from %d(%d, %d): g|%d, %d|\n",
	//	cell.NumMines(), cell.X(), cell.Y(),
	//	observation.numMines, len(observation.cells))
}

func (director *Director) End() {
	close(director.act)
}
