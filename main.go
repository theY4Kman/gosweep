package main

import (
	"github.com/faiface/pixel/pixelgl"
	"github.com/they4kman/gosweep/director/constraint"
	"github.com/they4kman/gosweep/game"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	gameConfig := game.NewGameConfig()
	gameConfig.Mode = game.Win7
	gameConfig.Width = 110
	gameConfig.Height = 60
	gameConfig.NumMines = 1200
	gameConfig.Director = &constraint.Director{}

	pixelgl.Run(func() {
		game.Run(gameConfig)
	})
}
