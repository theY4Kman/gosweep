package game

import (
	"fmt"
	"github.com/faiface/pixel"
	"sync"
	"sync/atomic"
)

type Cell struct {
	board *Board

	x, y     uint
	idx      uint
	numMines uint32

	isMine, isRevealed, isFlagged bool
	isLosingMine                  bool

	state   CellState
	sprite  *pixel.Sprite
	isDirty bool
}

func (cell *Cell) String() string {
	return fmt.Sprintf("Cell(%v, %v)", cell.x, cell.y)
}

func (cell *Cell) serialize() string {
	switch {
	case cell.isMine:
		switch {
		case cell.isLosingMine:
			return "*"
		case cell.isFlagged:
			return "F"
		default:
			return "O"
		}
	case cell.isFlagged:
		return "f"
	case cell.isRevealed:
		return "."
	default:
		return "#"
	}
}

func (cell *Cell) deserialize(c string, fresh bool) bool {
	switch c {
	case "*", "F", "O":
		cell.isMine = true

		switch c {
		case "*":
			if !fresh {
				cell.isLosingMine = true
				cell.isRevealed = true
				cell.setState(MineLosing)
			}
		case "F":
			cell.setFlagged(true)
		default:
			cell.setState(Unrevealed)
		}
	case "f":
		cell.setFlagged(true)
	case ".":
		cell.isRevealed = true
		// NOTE: this state will very likely be incorrect, until cell numbers are recalculated
		cell.setState(Empty)
		cell.board.markRevealed(cell)
	case "#":
		cell.isRevealed = false
		cell.setState(Unrevealed)
	default:
		return false
	}

	return true
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
		cell:   cell,
		action: Click,
	}
}

func (cell *Cell) RightClick() CellAction {
	return CellAction{
		cell:   cell,
		action: RightClick,
	}
}

func (cell *Cell) MiddleClick() CellAction {
	return CellAction{
		cell:   cell,
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
	cell.setFlagged(!cell.isFlagged)
}

func (cell *Cell) setFlagged(isFlagged bool) {
	cell.isFlagged = isFlagged

	if cell.isFlagged {
		cell.setState(Flag)
		cell.board.numFlags++
	} else {
		cell.setState(Unrevealed)
		cell.board.numFlags--
	}

	cell.board.markChanged(cell)
}

func (cell *Cell) setMine(isMine bool) {
	wasMine := cell.isMine
	if isMine == wasMine {
		return
	}
	cell.isMine = isMine

	var delta uint32

	if cell.isMine {
		cell.setState(Mine)
		delete(cell.board.remainingCells, cell)
		delta = 1
	} else {
		cell.setState(Unrevealed)
		cell.board.remainingCells.Add(cell)
		delta = ^uint32(0)
	}

	mineNeighbors := make(chan *Cell)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for cell := range mineNeighbors {
			atomic.AddUint32(&cell.numMines, delta)
		}
		wg.Done()
	}()

	cell.SendNeighbors(mineNeighbors)
	wg.Wait()

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
			cell.isLosingMine = true
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
		if !cell.isLosingMine {
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
