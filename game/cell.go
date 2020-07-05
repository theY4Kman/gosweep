package game

import (
	"fmt"
	"github.com/faiface/pixel"
)

type Cell struct {
	board *Board

	x, y     uint
	idx      uint
	numMines uint32

	isMine, isRevealed, isFlagged bool

	state   CellState
	sprite  *pixel.Sprite
	isDirty bool
}

func (cell *Cell) String() string {
	return fmt.Sprintf("Cell(%v, %v)", cell.x, cell.y)
}

func (cell *Cell) X() uint {
	return cell.x
}

func (cell *Cell) Y() uint {
	return cell.y
}

func (cell *Cell) IsRevealed() bool {
	return cell.isRevealed
}

func (cell *Cell) IsFlagged() bool {
	return cell.isFlagged
}

func (cell *Cell) NumMines() uint32 {
	return cell.numMines
}

func (cell *Cell) SelfNeighbors() <-chan *Cell {
	out := make(chan *Cell)
	go func() {
		out <- cell
		cell.SendNeighbors(out)
		close(out)
	}()
	return out
}

func (cell *Cell) Neighbors() <-chan *Cell {
	out := make(chan *Cell)
	go func() {
		cell.SendNeighbors(out)
		close(out)
	}()
	return out
}

func (cell *Cell) SendNeighbors(out chan<- *Cell) {
	board := cell.board

	isAtTopBorder := cell.y < 1
	isAtBottomBorder := cell.y >= board.height-1

	if cell.x >= 1 {
		out <- board.CellAt(cell.x-1, cell.y)

		if !isAtTopBorder {
			out <- board.CellAt(cell.x-1, cell.y-1)
		}
		if !isAtBottomBorder {
			out <- board.CellAt(cell.x-1, cell.y+1)
		}
	}

	if cell.x < board.width-1 {
		out <- board.CellAt(cell.x+1, cell.y)

		if !isAtTopBorder {
			out <- board.CellAt(cell.x+1, cell.y-1)
		}
		if !isAtBottomBorder {
			out <- board.CellAt(cell.x+1, cell.y+1)
		}
	}

	if !isAtTopBorder {
		out <- board.CellAt(cell.x, cell.y-1)
	}
	if !isAtBottomBorder {
		out <- board.CellAt(cell.x, cell.y+1)
	}
}

func (cell *Cell) Click() CellAction {
	return CellAction{
		cell: cell,
		action: Click,
	}
}

func (cell *Cell) RightClick() CellAction {
	return CellAction{
		cell: cell,
		action: RightClick,
	}
}

func (cell *Cell) MiddleClick() CellAction {
	return CellAction{
		cell: cell,
		action: MiddleClick,
	}
}

func (cell *Cell) click() {
	cell.board.actionGroup.Add(1)
	defer cell.board.actionGroup.Done()

	if !cell.board.hasClicked {
		cell.board.hasClicked = true

		if cell.board.mode == Win7 {
			cell.board.clearSurroundingMines(cell)
		}
	}

	if !cell.isRevealed {
		if !cell.isMine && cell.numMines == 0 {
			cell.cascadeEmpty()
		} else {
			cell.reveal()
		}
	}
}

func (cell *Cell) rightClick() {
	cell.board.actionGroup.Add(1)
	defer cell.board.actionGroup.Done()

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
	for neighbor := range cell.Neighbors() {
		if neighbor.isFlagged {
			numFlaggedNeighbors++
		}
	}

	if cell.numMines == numFlaggedNeighbors {
		for neighbor := range cell.Neighbors() {
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

	cell.board.markChanged(cell)
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

		cell.board.markChanged(cell)
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
			return cell.Neighbors()
		},
	)
}

func (cell *Cell) setState(state CellState) {
	if cell.state != state {
		cell.state = state
		cell.sprite = cellSprites[state]
		cell.isDirty = true
	}
}
