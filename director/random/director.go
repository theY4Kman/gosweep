package random

import (
	"github.com/they4kman/gosweep/game"
	"math/rand"
	"time"
)

type RandomDirector struct {
	act   chan struct{}
	done  chan struct{}
	board *game.Board
}

func (director *RandomDirector) Start(board *game.Board) {
	director.board = board
	director.act = make(chan struct{})
	director.done = make(chan struct{})

	go func() {
		unrevealedCells := make([]*game.Cell, director.board.NumCells())
		i := 0
		for cell := range director.board.Cells() {
			unrevealedCells[i] = cell
			i++
		}

		rand.Shuffle(len(unrevealedCells), func(i, j int) {
			unrevealedCells[i], unrevealedCells[j] = unrevealedCells[j], unrevealedCells[i]
		})

		for range director.act {
			for _, cell := range unrevealedCells {
				if !cell.IsRevealed() && !cell.IsFlagged() {
					cell.Click()
					break
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <- director.done:
				return
			default:
				time.Sleep(500 * time.Millisecond)
				director.act <- struct{}{}
			}
		}
	}()
}

func (director *RandomDirector) End() {
	director.done <- struct{}{}
	close(director.act)
}