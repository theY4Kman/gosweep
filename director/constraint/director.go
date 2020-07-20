package constraint

import (
	"fmt"
	"github.com/they4kman/gosweep/director/random"
	"github.com/they4kman/gosweep/game"
	"math"
	"reflect"
	"strings"
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

func (observation Observation) String() string {
	var cellsRepr strings.Builder
	lastIdx := len(observation.cells) - 1
	i := 0
	for cell := range observation.cells {
		cellsRepr.WriteString(fmt.Sprintf("(%d, %d)", cell.X(), cell.Y()))

		if i != lastIdx {
			cellsRepr.WriteString(", ")
		}
		i++
	}

	var originRepr string
	if observation.origin == nil {
		originRepr = "?"
	} else {
		originRepr = fmt.Sprintf("(%d, %d)", observation.origin.X(), observation.origin.Y())
	}

	return fmt.Sprintf("Obs[%8s, %d ε %s]", originRepr, observation.numMines, cellsRepr.String())
}

func (observation Observation) MineProbability() float32 {
	return float32(observation.numMines) / float32(len(observation.cells))
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
			actors := []func(actions chan<- game.CellAction){
				director.actDeliberate,
				director.actLowestProbability,
				director.actRandom,
			}

			for _, actor := range actors {
				actorActions := make(chan game.CellAction)
				foundAction := false

				wg := sync.WaitGroup{}
				wg.Add(1)
				go func() {
					for cellAction := range actorActions {
						foundAction = true
						actions <- cellAction
					}
					wg.Done()
				}()

				actor(actorActions)
				wg.Wait()

				if foundAction {
					break
				}
			}

			close(actions)
		}
	}()
}

func (director *Director) actRandom(actions chan<- game.CellAction) {
	randomDirector := &random.Director{}
	randomDirector.Init(director.board)
	randomDirector.Act(actions)
	randomDirector.End()
}

func (director *Director) actLowestProbability(actions chan<- game.CellAction) {
	lowestProbability := float32(math.Inf(1))
	numLowestProbabilityCells := 0

	cellProbabilities := make(map[*game.Cell]float32)
	for observation := range director.observations {
		probability := observation.MineProbability()

		for cell := range observation.cells {
			if probability < lowestProbability {
				lowestProbability = probability
				numLowestProbabilityCells = 0
			}

			pastProbability, hasPastProbability := cellProbabilities[cell]
			if !hasPastProbability || probability < pastProbability {
				cellProbabilities[cell] = probability
			}

			if probability <= lowestProbability && probability != pastProbability {
				numLowestProbabilityCells++
			}
		}
	}

	if len(cellProbabilities) > 0 {
		lowestProbabilityCells := make([]*game.Cell, numLowestProbabilityCells)
		i := 0
		for cell, probability := range cellProbabilities {
			if probability <= lowestProbability {
				lowestProbabilityCells[i] = cell
				i++

				director.board.AddAnnotation(game.Annotation{
					Type: game.AnnotateHighlightYellow,
					Cell: cell,
				})
			}
		}

		director.board.Rand().Shuffle(len(lowestProbabilityCells), func(i, j int) {
			lowestProbabilityCells[i], lowestProbabilityCells[j] = lowestProbabilityCells[j], lowestProbabilityCells[i]
		})

		actions <- lowestProbabilityCells[0].Click()
	}

	close(actions)
}

func (director *Director) actDeliberate(actions chan<- game.CellAction) {
	director.observationsLock.Lock()
	defer director.observationsLock.Unlock()

	wg := sync.WaitGroup{}
	findDeliberateActions := func(observations <-chan *Observation) {
		defer wg.Done()

		for observation := range observations {
			if observation.numMines == len(observation.cells) {
				for cell := range observation.cells {
					actions <- cell.RightClick()
				}

				observation.cells = nil
				observation.numMines = 0

			} else if observation.numMines == 0 {
				for cell := range observation.cells {
					actions <- cell.Click()
				}

				observation.cells = nil
				observation.numMines = 0
			}
		}
	}

	observations := make(chan *Observation)
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go findDeliberateActions(observations)
	}

	for observation := range director.observations {
		observations <- observation
	}
	close(observations)

	wg.Wait()
	close(actions)
}

func (director *Director) Act(actions chan<- game.CellAction) {
	if director.act != nil {
		director.act <- actions
	}
}

func (director *Director) CellChanges(changes <-chan *game.Cell) {
	for cell := range changes {
		if cell.IsRevealed() {
			director.cellRevealed(cell)
		}

		if cellObservations, ok := director.observationsByCell[cell]; ok {
			for observation := range cellObservations {
				if observation.numMines == 0 || len(observation.cells) == 0 {
					delete(cellObservations, observation)
				} else if cell.IsFlagged() {
					director.removeObservationCell(observation, cell)
					observation.numMines--
				} else if cell.IsRevealed() {
					director.removeObservationCell(observation, cell)
				}
			}
		}
	}

	// Simplify/split observations
	for i := 0; i < 4; i++ {
		director.simplifyObservations()
	}
}

func (director *Director) simplifyObservations() {
	for observation := range director.observations {
		if len(observation.cells) == 0 || observation.origin == nil {
			director.removeObservation(observation)
			continue
		}

		visited := make(map[*Observation]struct{})

		for cell := range observation.cells {
			for intersectingObs := range director.observationsByCell[cell] {
				if intersectingObs == observation {
					continue
				}
				if _, alreadyVisited := visited[intersectingObs]; alreadyVisited {
					continue
				}
				visited[intersectingObs] = struct{}{}

				isSubset := true
				sharedCells := make(map[*game.Cell]struct{})
				for cell := range observation.cells {
					if _, isInIntersectingObs := intersectingObs.cells[cell]; isInIntersectingObs {
						sharedCells[cell] = struct{}{}
					} else {
						isSubset = false
					}
				}

				if isSubset {
					splitObs := Observation{
						numMines: intersectingObs.numMines - observation.numMines,
						cells:    make(map[*game.Cell]struct{}),
					}
					for cell := range intersectingObs.cells {
						if _, isInObs := observation.cells[cell]; !isInObs {
							splitObs.cells[cell] = struct{}{}
						}
					}

					director.addObservation(&splitObs)
				} else if observation.numMines == 1 && len(sharedCells) > 1 {
					leftOnlyCells := make(map[*game.Cell]struct{})
					for cell := range intersectingObs.cells {
						if _, isShared := sharedCells[cell]; !isShared {
							leftOnlyCells[cell] = struct{}{}
						}
					}

					occludedMines := intersectingObs.numMines - observation.numMines

					if occludedMines == len(leftOnlyCells) {
						occludedObs := Observation{
							numMines: occludedMines,
							cells:    leftOnlyCells,
						}

						director.addObservation(&occludedObs)
					}
				}
			}
		}
	}
}

func (director *Director) addObservation(observation *Observation) {
	// Don't add vacuous observations
	if len(observation.cells) == 0 {
		return
	}

	dupeVisited := make(map[*Observation]struct{})
	for cell := range observation.cells {
		var cellObservations map[*Observation]struct{}
		var exists bool
		if cellObservations, exists = director.observationsByCell[cell]; !exists {
			cellObservations = make(map[*Observation]struct{})
			director.observationsByCell[cell] = cellObservations
		}

		for otherObs := range cellObservations {
			if _, hasVisited := dupeVisited[otherObs]; hasVisited {
				continue
			}
			dupeVisited[otherObs] = struct{}{}

			// Don't add duplicates
			if reflect.DeepEqual(observation.cells, otherObs.cells) {
				return
			}
		}

		cellObservations[observation] = struct{}{}
	}

	director.observations[observation] = struct{}{}
}

func (director *Director) removeObservation(observation *Observation) {
	delete(director.observations, observation)

	for cell := range observation.cells {
		delete(director.observationsByCell[cell], observation)
	}
}

func (director *Director) removeObservationCell(observation *Observation, cell *game.Cell) {
	delete(observation.cells, cell)
	delete(director.observationsByCell[cell], observation)
}

func (director *Director) cellRevealed(cell *game.Cell) {
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

	director.addObservation(&observation)

	//XXX///////////////////////////////////////////////////////////////////////////////////////////
	//fmt.Fprintf(os.Stdout,
	//	"Found observation, from %d(%d, %d): g|%d, %d|\n",
	//	cell.NumMines(), cell.X(), cell.Y(),
	//	observation.numMines, len(observation.cells))
}

func (director *Director) End() {
	act := director.act
	director.act = nil
	close(act)
}
