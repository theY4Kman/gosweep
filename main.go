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
	pixelgl.Run(func() {
		game.RunDirector(&random.RandomDirector{})
	})
}
