package constraint

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/they4kman/gosweep/director/random"
	"github.com/they4kman/gosweep/game"
	"github.com/they4kman/gosweep/util/collections"
	"math"
	"reflect"
	"strings"
	"sync"
)

type Director struct {
	game.BaseDirector

	board *game.Board

	act chan chan<- game.CellAction

	observations       collections.Set[*Observation]
	observationsByCell map[*game.Cell]collections.Set[*Observation]
	observationsLock   *sync.Mutex
}

type Observation struct {
	origin   *game.Cell
	numMines int
	cells    collections.Set[*game.Cell]
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

	director.observations = make(collections.Set[*Observation])
	director.observationsByCell = make(map[*game.Cell]collections.Set[*Observation])

	if director.observationsLock == nil {
		director.observationsLock = &sync.Mutex{}
	}

	go func() {
		for actions := range director.act {
			actors := []func(actions chan<- game.CellAction){
				director.actDeliberate,
				director.actEndGame,
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

func (director *Director) actEndGame(actions chan<- game.CellAction) {
	if director.board.NumMinesRemaining() == 1 {
		director.observationsLock.Lock()
		defer director.observationsLock.Unlock()

		var sharedCells collections.Set[*game.Cell] = nil

		for observation := range director.observations {
			if observation.numMines != 1 {
				continue
			}

			if sharedCells == nil {
				sharedCells = make(collections.Set[*game.Cell])
				for cell := range observation.cells {
					sharedCells.Add(cell)
				}
			} else {
				for cell := range sharedCells {
					if _, isShared := observation.cells[cell]; !isShared {
						delete(sharedCells, cell)
					}
				}
			}
		}

		if len(sharedCells) == 1 {
			for cell := range sharedCells {
				actions <- cell.RightClick()
			}
		}
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
	logrus.Debug("Received new cell changes")

	for cell := range changes {
		if cell.IsRevealed() {
			director.cellRevealed(cell)
		}

		director.observationsLock.Lock()
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
		director.observationsLock.Unlock()
	}

	// Simplify/split observations
	director.clearInferredObservations()
	director.simplifyObservations()
}

func (director *Director) clearInferredObservations() {
	for observation := range director.observations {
		if len(observation.cells) == 0 || observation.origin == nil {
			director.removeObservation(observation)
			continue
		}
	}
}

func (director *Director) simplifyObservations() {
	logrus.Debug("Simplifying observations")

	for observation := range director.observations {
		if len(observation.cells) == 0 {
			director.removeObservation(observation)
			continue
		}

		visited := make(collections.Set[*Observation])

		for cell := range observation.cells {
			for intersectingObs := range director.observationsByCell[cell] {
				if intersectingObs == observation {
					continue
				}
				if visited.Contains(intersectingObs) {
					continue
				}
				visited.Add(intersectingObs)

				sharedCells, isSubset := observation.cells.IntersectionEx(intersectingObs.cells)

				if isSubset {
					splitObs := Observation{
						numMines: intersectingObs.numMines - observation.numMines,
						cells:    intersectingObs.cells.Difference(observation.cells),
					}
					director.addObservation(&splitObs)

					logrus.Debugf(
						"Subset:     %s\n  Superset: %s\n  Split:    %s",
						observation, intersectingObs, splitObs)

				} else if observation.numMines == 1 && len(sharedCells) > 1 {
					// Only cells in intersectingObs
					leftOnlyCells := intersectingObs.cells.Difference(sharedCells)
					occludedMines := intersectingObs.numMines - observation.numMines

					if occludedMines == len(leftOnlyCells) {
						occludedObs := Observation{
							numMines: occludedMines,
							cells:    leftOnlyCells,
						}

						director.addObservation(&occludedObs)

						logrus.Debugf(
							"Limiting:   %s\n  Limited:  %s\n  Occluded:    %s",
							observation, intersectingObs, occludedObs)
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

	dupeVisited := make(collections.Set[*Observation])
	for cell := range observation.cells {
		var cellObservations collections.Set[*Observation]
		var exists bool
		if cellObservations, exists = director.observationsByCell[cell]; !exists {
			cellObservations = make(collections.Set[*Observation])
			director.observationsByCell[cell] = cellObservations
		}

		for otherObs := range cellObservations {
			if dupeVisited.Contains(otherObs) {
				continue
			}
			dupeVisited.Add(otherObs)

			// Don't add duplicates
			if reflect.DeepEqual(observation.cells, otherObs.cells) {
				return
			}
		}

		cellObservations.Add(observation)
	}

	director.observations.Add(observation)
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
		cells:    make(collections.Set[*game.Cell]),
	}

	for neighbor := range cell.Neighbors() {
		if !neighbor.IsRevealed() {
			if neighbor.IsFlagged() {
				observation.numMines--
			} else {
				observation.cells.Add(neighbor)
			}
		}
	}

	if len(observation.cells) == 0 {
		return
	}

	director.observationsLock.Lock()
	defer director.observationsLock.Unlock()

	director.addObservation(&observation)

	logrus.Debugf(
		"Found observation, from %d(%d, %d): g|%d, %d|",
		cell.NumMines(), cell.X(), cell.Y(),
		observation.numMines, len(observation.cells))
}

func (director *Director) End() {
	act := director.act
	director.act = nil
	close(act)
}
