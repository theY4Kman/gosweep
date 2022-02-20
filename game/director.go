package game

import "time"

type Action int

const (
	Click Action = iota
	MiddleClick
	RightClick
)

type CellAction struct {
	cell   *Cell
	action Action
}

func (cellAction CellAction) perform() {
	switch cellAction.action {
	case Click:
		cellAction.cell.click()
	case MiddleClick:
		cellAction.cell.middleClick()
	case RightClick:
		cellAction.cell.rightClick()
	default:
	}
}

type AnnotationType int

const (
	AnnotateClick           = AnnotationType(Click)
	AnnotateMiddleClick     = AnnotationType(MiddleClick)
	AnnotateRightClick      = AnnotationType(RightClick)
	AnnotateHighlightYellow = iota
)

type Annotation struct {
	Type AnnotationType
	Cell *Cell

	frame      int64
	firstShown time.Time
}

type Director interface {
	// Initialize the director
	Init(*Board)

	// Perform zero or more actions, by sending CellAction messages on the
	// channel, then closing it.
	Act(actions chan<- CellAction)

	// Request calls to Act() periodically, by sending messages onto
	// the act channel, until a message is passed to the done channel
	actContinuously(tickRate time.Duration, act chan<- struct{}, done <-chan struct{})

	// Called before Act() with a channel containing all the Cells that have
	// changed since the last call to Act()
	CellChanges(changes <-chan *Cell)

	// Cleanup
	End()
}

type BaseDirector struct{}

func (director *BaseDirector) Init(*Board) {
}

func (director *BaseDirector) Act(chan<- CellAction) {
}

func (director *BaseDirector) actContinuously(tickRate time.Duration, act chan<- struct{}, done <-chan struct{}) {
	go func() {
		tick := time.Tick(tickRate)

		for {
			select {
			case <-done:
				return
			case <-tick:
				act <- struct{}{}
			default:
			}
		}
	}()
}

func (director *BaseDirector) CellChanges(changes <-chan *Cell) {
}

func (director *BaseDirector) End() {
}
