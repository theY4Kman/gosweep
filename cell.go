package main

import (
	"fmt"
	"github.com/faiface/pixel"
)

type Cell struct {
	board *Board

	sprite   *pixel.Sprite
	x, y     uint
	idx      uint
	numMines uint32

	isMine, isRevealed, isFlagged bool

	state   CellState
	isDirty bool
}

func (cell *Cell) setState(state CellState) {
	if cell.state != state {
		cell.state = state
		cell.sprite = cellSprites[state]
		cell.isDirty = true
	}
}

func (cell *Cell) String() string {
	return fmt.Sprintf("Cell(%v, %v)", cell.x, cell.y)
}

func (cell *Cell) neighbors() <-chan *Cell {
	out := make(chan *Cell)
	go func() {
		cell.sendNeighbors(out)
		close(out)
	}()
	return out
}

func (cell *Cell) sendNeighbors(out chan<- *Cell) {
	board := cell.board

	isAtTopBorder := cell.y < 1
	isAtBottomBorder := cell.y >= board.height-1

	if cell.x >= 1 {
		out <- board.cellAt(cell.x-1, cell.y)

		if !isAtTopBorder {
			out <- board.cellAt(cell.x-1, cell.y-1)
		}
		if !isAtBottomBorder {
			out <- board.cellAt(cell.x-1, cell.y+1)
		}
	}

	if cell.x < board.width-1 {
		out <- board.cellAt(cell.x+1, cell.y)

		if !isAtTopBorder {
			out <- board.cellAt(cell.x+1, cell.y-1)
		}
		if !isAtBottomBorder {
			out <- board.cellAt(cell.x+1, cell.y+1)
		}
	}

	if !isAtTopBorder {
		out <- board.cellAt(cell.x, cell.y-1)
	}
	if !isAtBottomBorder {
		out <- board.cellAt(cell.x, cell.y+1)
	}
}

func (cell *Cell) click() {
	if !cell.isRevealed {
		if !cell.isMine && cell.numMines == 0 {
			cell.cascadeEmpty()
		} else {
			cell.reveal()
		}
	}
}

func (cell *Cell) rightClick() {
	if !cell.isRevealed {
		cell.toggleFlagged()
	}
}

func (cell *Cell) middleClick() {
	if !cell.isRevealed {
		return
	}
	if cell.isFlagged {
		return
	}

	numFlaggedNeighbors := uint32(0)
	for neighbor := range cell.neighbors() {
		if neighbor.isFlagged {
			numFlaggedNeighbors++
		}
	}

	if cell.numMines == numFlaggedNeighbors {
		for neighbor := range cell.neighbors() {
			neighbor.click()
		}
	}
}

func (cell *Cell) toggleFlagged() {
	cell.isFlagged = !cell.isFlagged
	if cell.isFlagged {
		cell.setState(Flag)
		cell.board.numFlags++
	} else {
		cell.setState(Unrevealed)
		cell.board.numFlags--
	}
}

func (cell *Cell) reveal() {
	if cell.isFlagged {
		return
	}

	if !cell.isRevealed {
		cell.isRevealed = true

		if cell.isMine {
			cell.setState(MineLosing)
			cell.board.lose()
		} else {
			cell.setState(CellState(cell.numMines))
		}

		cell.board.markRevealed(cell)
	}
}

func (cell *Cell) revealLost() {
	if cell.isFlagged {
		if !cell.isMine {
			cell.setState(FlagWrong)
		}
	} else if cell.isMine {
		if cell.state != MineLosing {
			cell.setState(MineUnrevealed)
		}
	}
}

func (cell *Cell) cascadeEmpty() {
	flood(
		cell,
		func(cell *Cell) {
			cell.reveal()
		},
		func(cell *Cell) <-chan *Cell {
			return cell.neighbors()
		},
	)
}
