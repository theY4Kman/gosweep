package game

import (
	"fmt"
	"github.com/faiface/pixel/imdraw"
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
	return createBoard(config.Width, config.Height, config.NumMines, config.Mode, config.Director)
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
	topRight := win.Bounds().Max
	boardTopLeft := topLeft.Sub(pixel.V(0, float64(headerHeight)))

	basicAtlas := text.NewAtlas(basicfont.Face7x13, text.ASCII)

	scoreText := text.New(topLeft.Add(pixel.V(20, -30)), basicAtlas)
	scoreText.Color = colornames.Black

	cellPosText := text.New(topRight.Add(pixel.V(-60, -30)), basicAtlas)
	cellPosText.Color = colornames.Darkcyan
	var hoveredCell *Cell

	board := config.createBoard()
	board.startGame()

	var (
		frames = 0
		second = time.Tick(time.Second)
	)

	bgColor := colornames.Gainsboro
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

		if win.MouseInsideWindow() {
			x, y := board.screenToGridCoords(win.MousePosition())
			hoveredCell = board.CellAt(x, y)
		} else {
			hoveredCell = nil
		}

		cellPosText.Clear()
		if hoveredCell != nil {
			fmt.Fprintf(cellPosText, "(%d, %d)", hoveredCell.x, hoveredCell.y)
			cellPosText.Draw(win, pixel.IM)
		}

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

		if board.directorAnnotations != nil {
			imd := imdraw.New(nil)

			now := time.Now()
			board.directorAnnotationLock.Lock()
			for cellAction, expiresAt := range board.directorAnnotations {
				if now.After(expiresAt) {
					if board.state == Ongoing || board.state == Won {
						delete(board.directorAnnotations, cellAction)
						continue
					}
				}

				cell := cellAction.cell
				start := boardTopLeft.Add(
					pixel.V(
						float64(cellWidth*cell.x),
						-float64(cellWidth*(cell.y+1)),
					),
				)
				end := start.Add(pixel.V(cellWidth, cellWidth))
				baseColor := pixel.Alpha(0)

				switch cellAction.action {
				case Click:
					baseColor = pixel.RGB(1, 0, 0)
				case RightClick:
					baseColor = pixel.RGB(0, 0, 1)
				case MiddleClick:
					baseColor = pixel.RGB(0, 1, 0)
				}

				imd.Color = baseColor.Mul(pixel.Alpha(0.5))
				imd.Push(start, end)
				imd.Rectangle(0)  // 0 = filled
			}

			board.directorAnnotationLock.Unlock()
			imd.Draw(win)
		}

		if !board.canPlay() {
			if win.JustPressed(pixelgl.KeyEnter) {
				board = config.createBoard()
				board.startGame()
			}

			continue
		}

		if win.JustPressed(pixelgl.MouseButtonLeft) || win.JustPressed(pixelgl.MouseButtonRight) || win.JustPressed(pixelgl.MouseButtonMiddle) {
			if hoveredCell != nil {
				if win.JustPressed(pixelgl.MouseButtonLeft) {
					hoveredCell.click()
				}
				if win.JustPressed(pixelgl.MouseButtonRight) {
					hoveredCell.rightClick()
				}
				if win.JustPressed(pixelgl.MouseButtonMiddle) {
					hoveredCell.middleClick()
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
