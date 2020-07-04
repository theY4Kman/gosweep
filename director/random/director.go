package random

import (
	"github.com/they4kman/gosweep/game"
	"math/rand"
	"time"
)

type Director struct {
	act   chan struct{}
	done  chan struct{}
	board *game.Board
}

func (director *Director) Init(board *game.Board) {
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
}

func (director *Director) Act() {
	director.act <- struct{}{}
}

func (director *Director) ActContinuously() {
	go func() {
		tick := time.Tick(500 * time.Millisecond)

		for {
			select {
			case <- director.done:
				return
			case <- tick:
				director.Act()
			default:
			}
		}
	}()
}

func (director *Director) End() {
	director.done <- struct{}{}
	close(director.act)
}