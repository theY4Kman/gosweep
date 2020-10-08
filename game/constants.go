package game

type CellState int
type BoardState int

const (
	Unrevealed CellState = iota - 1
	Empty
	Number1
	Number2
	Number3
	Number4
	Number5
	Number6
	Number7
	Number8
	Flag
	FlagWrong
	Mine
	MineUnrevealed
	MineLosing
)

var CellStates = []CellState{
	Unrevealed,
	Empty,
	Number1,
	Number2,
	Number3,
	Number4,
	Number5,
	Number6,
	Number7,
	Number8,
	Flag,
	FlagWrong,
	Mine,
	MineUnrevealed,
	MineLosing,
}

const (
	cellWidth = 16
)

const (
	Lost = iota
	Won
	Ongoing
	Paused
)
