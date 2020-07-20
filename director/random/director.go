package random

import (
	"github.com/they4kman/gosweep/game"
)

type Director struct {
	game.BaseDirector

	act chan chan<- game.CellAction
}

func (director *Director) Init(board *game.Board) {
	director.act = make(chan chan<- game.CellAction)

	go func() {
		unrevealedCells := make([]*game.Cell, board.NumCells())
		i := 0
		for cell := range board.Cells() {
			unrevealedCells[i] = cell
			i++
		}

		board.Rand().Shuffle(len(unrevealedCells), func(i, j int) {
			unrevealedCells[i], unrevealedCells[j] = unrevealedCells[j], unrevealedCells[i]
		})

		for actions := range director.act {
			for _, cell := range unrevealedCells {
				if !cell.IsRevealed() && !cell.IsFlagged() {
					actions <- cell.Click()
					break
				}
			}

			close(actions)
		}
	}()
}

func (director *Director) Act(actions chan<- game.CellAction) {
	director.act <- actions
}

func (director *Director) End() {
	close(director.act)
}
