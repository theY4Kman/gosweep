/**
 * Copyright 2015 Zach Kanzler
 */

package main

import (
	"image"
	"log"
	"time"

	_ "image/png"

	"golang.org/x/mobile/app"
	"golang.org/x/mobile/asset"
	"golang.org/x/mobile/event"
	"golang.org/x/mobile/exp/app/debug"
	"golang.org/x/mobile/exp/f32"
	"golang.org/x/mobile/exp/sprite"
	"golang.org/x/mobile/exp/sprite/clock"
	"golang.org/x/mobile/exp/sprite/glsprite"
	"golang.org/x/mobile/geom"
	"golang.org/x/mobile/gl"
	"math/rand"
)

const (
	cellWidth    = 12
	marginTop    = 12
	marginBottom = 20
)

type BoardState int

const (
	Lost = iota
	Won
	Ongoing
)

type CellState int

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

var texnames = map[CellState]string{
	Unrevealed:     "unrevealed.png",
	Empty:          "empty.png",
	Number1:        "number1.png",
	Number2:        "number2.png",
	Number3:        "number3.png",
	Number4:        "number4.png",
	Number5:        "number5.png",
	Number6:        "number6.png",
	Number7:        "number7.png",
	Number8:        "number8.png",
	Flag:           "flag.png",
	FlagWrong:      "flag_wrong.png",
	Mine:           "mine.png",
	MineUnrevealed: "mine_unrevealed.png",
	MineLosing:     "mine_losing.png",
}

var texs = map[CellState]sprite.SubTex{}

type cell struct {
	Node         *sprite.Node
	X, Y         int
	NumNeighbors int

	isMine, isRevealed, isFlagged bool
	state                         CellState
}

type board struct {
	Width, Height int // in number of cells
	Cells         [][]cell

	state BoardState
}

func (b board) NumMines() int {
	// Number of mines the board should have, based on 16x30 having 99 mines
	return (b.Width * b.Height * 99) / (16 * 30)
}

var (
	startTime = time.Now()
	eng       = glsprite.Engine()
	scene     *sprite.Node
	brd       = board{state: Ongoing}
)

func main() {
	app.Run(app.Callbacks{
		Draw:  draw,
		Touch: touch,
	})
}

func (c *cell) relCell(d_x, d_y int) *cell {
	x := c.X + d_x
	y := c.Y + d_y
	if x < 0 || x >= brd.Width || y < 0 || y >= brd.Height {
		return nil
	}
	return brd.CellAt(x, y)
}

func (c *cell) SetState(state CellState) {
	c.state = state
	eng.SetSubTex(c.Node, texs[c.state])
}

func (c *cell) GetState() CellState {
	return c.state
}

func (c *cell) SetMine() {
	c.isMine = true
}

func (c *cell) IsMine() bool {
	return c.isMine
}

func (c *cell) Reveal() {
	c.isRevealed = true

	if c.IsMine() {
		c.SetState(MineLosing)
		brd.Lose()
	} else {
		c.SetState(CellState(c.NumNeighbors))
	}
}

func (c *cell) IsRevealed() bool {
	return c.isRevealed
}

func (c *cell) SetFlagged(isFlagged bool) {
	c.isFlagged = isFlagged
}

func (c *cell) IsFlagged() bool {
	return c.isFlagged
}

func (c *cell) cascadeEmpty() {
	_cascadeEmpty(c)
}

func _cascadeEmpty(cell *cell) {
	if !cell.IsMine() && !cell.IsFlagged() {
		cell.Reveal()
		for _, neighbor := range cell.Neighbors() {
			if neighbor.IsMine() || neighbor.IsRevealed() {
				continue
			}
			if neighbor.NumNeighbors == 0 {
				_cascadeEmpty(neighbor)
			} else {
				neighbor.Reveal()
			}
		}
	}
}

func neighborCoords() []geom.Point {
	return []geom.Point{
		geom.Point{-1, -1},
		geom.Point{0, -1},
		geom.Point{1, -1},
		geom.Point{1, 0},
		geom.Point{1, 1},
		geom.Point{0, 1},
		geom.Point{-1, 1},
		geom.Point{-1, 0},
	}
}

func (c *cell) Neighbors() []*cell {
	neighbors := make([]*cell, 8)
	total := 0
	for _, pt := range neighborCoords() {
		neighbor := c.relCell(int(pt.X), int(pt.Y))
		if neighbor != nil {
			neighbors[total] = neighbor
			total++
		}
	}
	return neighbors[:total]
}

func (c *cell) WasTouched() {
	if !c.IsRevealed() && !c.IsFlagged() {
		c.Reveal()

		if c.NumNeighbors == 0 {
			c.cascadeEmpty()
		}
	}
}

func (c *cell) NeighboringMines() []*cell {
	neighbors := c.Neighbors()
	mines := make([]*cell, len(neighbors))
	total := 0
	for _, neighbor := range neighbors {
		if neighbor.IsMine() {
			mines[total] = neighbor
			total++
		}
	}
	return mines[:total]
}

func (b board) Lose() {
	b.state = Lost
}

func (b board) TranslateScreenPoint(loc geom.Point) (x, y int) {
	x = int(loc.X / cellWidth)
	y = int((loc.Y - marginTop) / cellWidth)
	return
}

func (b board) CellAt(x, y int) *cell {
	return &b.Cells[x][y]
}

func draw(c event.Config) {
	if scene == nil {
		loadScene(c)
	}

	gl.ClearColor(1, 1, 1, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)
	now := clock.Time(time.Since(startTime) * 60 / time.Second)
	eng.Render(scene, now, c)
	debug.DrawFPS(c)
}

func touch(e event.Touch, c event.Config) {
	if e.Change == event.ChangeOn {
		x, y := brd.TranslateScreenPoint(e.Loc)
		touched := brd.CellAt(x, y)
		touched.WasTouched()
	}
}

func newNode() *sprite.Node {
	n := &sprite.Node{}
	eng.Register(n)
	scene.AppendChild(n)
	return n
}

func newCell(x, y int) cell {
	cell := cell{
		Node: newNode(),
		X:    x,
		Y:    y,
	}
	cell.SetState(Unrevealed)
	eng.SetTransform(cell.Node, f32.Affine{
		{cellWidth, 0, float32(x * cellWidth)},
		{0, cellWidth, float32(y * cellWidth)},
	})
	return cell
}

func loadScene(c event.Config) {
	scene = &sprite.Node{}
	eng.Register(scene)
	eng.SetTransform(scene, f32.Affine{
		{1, 0, 0},
		{0, 1, marginTop},
	})

	loadTextures()

	brd.Width = int(c.Width) / cellWidth
	brd.Height = int(float32(c.Height)-float32(marginTop+marginBottom)/float32(c.PixelsPerPt)) / cellWidth

	brd.Cells = make([][]cell, brd.Width)
	for x := 0; x < brd.Width; x++ {
		brd.Cells[x] = make([]cell, brd.Height)
		for y := 0; y < brd.Height; y++ {
			brd.Cells[x][y] = newCell(x, y)
		}
	}

	numMines := brd.NumMines()
	for cnt, i := range rand.Perm(brd.Width * brd.Height) {
		cell := &brd.Cells[i/brd.Height][i%brd.Height]
		if cnt <= numMines {
			cell.SetMine()
		} else {
			cell.NumNeighbors = len(cell.NeighboringMines())
		}
	}
}

func loadTextures() {
	for state, name := range texnames {
		texs[state] = loadTexture(name)
	}
}

func loadTexture(name string) sprite.SubTex {
	f, err := asset.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	t, err := eng.LoadTexture(img)
	if err != nil {
		log.Fatal(err)
	}

	return sprite.SubTex{t, image.Rect(0, 0, 16, 16)}
}
