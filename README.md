# gosweep

gosweep is a minesweeper game, written in Go using the [pixel game library](https://github.com/faiface/pixel), with an automatic player built-in

(Originally, this project began as a hack day project at GopherCon 2015; an Android minesweeper clone using [gomobile](http://godoc.org/golang.org/x/mobile/cmd/gomobile))


# auto-gosweep

To have Minesweeper played _for_ you, pass `--director`:
```bash
gosweep --director
```

There are pretty colours showing the actions the director took. Red is a left click (reveal), blue is a right click (flag), and yellow means the director guessed — it chose one of the yellow cells at random.

![Director Example](https://user-images.githubusercontent.com/33840/95430181-6350bc80-0919-11eb-993d-d0ce904adacd.gif)

Since the game and director are written in Go, it's FAST, and can handle large boards pretty well.

![Director Example with Large Board](https://user-images.githubusercontent.com/33840/95430708-21744600-091a-11eb-9fa3-fccb4653529c.gif)

... and that speed is artificially limited. With the 25ms tickrate removed, games are near-instantaneous

![Director Example with No Artificial Tick Rate](https://user-images.githubusercontent.com/33840/95431579-63ea5280-091b-11eb-8f17-cb3edfb89e4b.gif)
