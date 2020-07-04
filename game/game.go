package game

import (
	"fmt"
	"github.com/faiface/pixel/text"
	"golang.org/x/image/font/basicfont"
	"image"
	"os"
	"time"

	_ "image/png"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"golang.org/x/image/colornames"
)

var cellSprites = map[CellState]*pixel.Sprite{}

type GameMode int

const (
	Classic GameMode = iota
	Win7
)

type GameConfig struct {
	Width, Height uint
	NumMines      uint
	Mode          GameMode

	Director Director
}

func NewGameConfig() GameConfig {
	return GameConfig{
		Width:    30,
		Height:   16,
		NumMines: 99,
		Mode: Classic,
		Director: nil,
	}
}

func (config GameConfig) createBoard() *Board {
	return createBoard(config.Width, config.Height, config.NumMines, config.Director)
}

func Run(config GameConfig) {
	headerHeight := uint(50)

	cfg := pixelgl.WindowConfig{
		Title: "gosweep",
		Bounds: pixel.R(
			0, 0,
			float64(config.Width*cellWidth), float64(config.Height*cellWidth+headerHeight),
		),
	}
	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	spritesheet := loadSpritesheet()
	batch := pixel.NewBatch(&pixel.TrianglesData{}, spritesheet)

	topLeft := win.Bounds().Vertices()[1]
	boardTopLeft := topLeft.Sub(pixel.V(0, float64(headerHeight)))

	scoreAtlas := text.NewAtlas(basicfont.Face7x13, text.ASCII)
	scoreText := text.New(topLeft.Add(pixel.V(20, -30)), scoreAtlas)
	scoreText.Color = colornames.Black

	board := config.createBoard()
	hasClicked := false

	board.startGame()

	var (
		frames = 0
		second = time.Tick(time.Second)
	)

	bgColor := colornames.Lightgray
	for !win.Closed() {
		win.Update()
		win.Clear(bgColor)

		frames++
		select {
		case <-second:
			win.SetTitle(fmt.Sprintf("%s | FPS: %d", cfg.Title, frames))
			frames = 0
		default:
		}

		scoreText.Clear()
		scoreText.Color = colornames.Black

		fmt.Fprintf(scoreText, "%03d", board.numMines-board.numFlags)
		if !board.canPlay() {
			var boardState string
			if board.state == Won {
				boardState = "WIN!"
				scoreText.Color = colornames.Green
			} else if board.state == Lost {
				boardState = "LOSE :("
				scoreText.Color = colornames.Red
			}

			fmt.Fprintf(scoreText, "   %s", boardState)
		}
		scoreText.Draw(win, pixel.IM)

		batch.Clear()
		for y, row := range board.cells {
			rowStart := boardTopLeft.Add(pixel.V(cellWidth/2, -float64(cellWidth/2+cellWidth*y)))

			for x, cell := range row {
				cellPos := rowStart.Add(pixel.V(float64(cellWidth*x), 0))

				if cell.isDirty {
					cell.sprite.Draw(batch, pixel.IM.Moved(cellPos))
					cell.isDirty = false
				}
			}
		}
		batch.Draw(win)

		if !board.canPlay() {
			if win.JustPressed(pixelgl.KeyEnter) {
				board = config.createBoard()
				hasClicked = false
				board.startGame()
			}

			continue
		}

		if win.JustPressed(pixelgl.MouseButtonLeft) || win.JustPressed(pixelgl.MouseButtonRight) || win.JustPressed(pixelgl.MouseButtonMiddle) {
			x, y := board.screenToGridCoords(win.MousePosition())
			cell := board.CellAt(x, y)

			if cell != nil {
				if win.JustPressed(pixelgl.MouseButtonLeft) {
					if !hasClicked {
						hasClicked = true

						if config.Mode == Win7 {
							board.clearSurroundingMines(cell)
						}
					}

					cell.Click()
				}
				if win.JustPressed(pixelgl.MouseButtonRight) {
					cell.RightClick()
				}
				if win.JustPressed(pixelgl.MouseButtonMiddle) {
					cell.MiddleClick()
				}
			}
		}
	}
}

func loadSpritesheet() pixel.Picture {
	spritesheet, err := loadPicture("assets/spritesheet.png")
	if err != nil {
		panic(err)
	}

	x1, x2 := float64(0), float64(cellWidth)
	y2 := spritesheet.Bounds().Max.Y
	for _, state := range CellStates {
		frame := pixel.R(x1, y2-cellWidth, x2, y2)
		cellSprites[state] = pixel.NewSprite(spritesheet, frame)

		y2 -= cellWidth
	}

	return spritesheet
}

func loadPicture(path string) (pixel.Picture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return pixel.PictureDataFromImage(img), nil
}
