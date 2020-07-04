package main

import (
	"github.com/faiface/pixel/pixelgl"
	"github.com/they4kman/gosweep/director/random"
	"github.com/they4kman/gosweep/game"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	gameConfig := game.NewGameConfig()
	gameConfig.Mode = game.Win7
	gameConfig.Director = &random.Director{}

	pixelgl.Run(func() {
		game.Run(gameConfig)
	})
}
