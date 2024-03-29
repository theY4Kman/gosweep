package game

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/faiface/pixel/imdraw"
	"github.com/faiface/pixel/text"
	"golang.org/x/image/font/basicfont"
	"image"
	"math"
	"os"
	"strings"
	"time"

	_ "image/png"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"golang.org/x/image/colornames"
)

//go:embed assets/spritesheet.png
var spritesheetPNG []byte
var cellSprites = map[CellState]*pixel.Sprite{}

type GameMode int

const (
	Classic GameMode = iota
	Win7
)

type GameConfig struct {
	Width, Height uint
	NumMines      uint
	Fullscreen    bool
	MineDensity   float64
	Mode          GameMode

	Seed int64

	// Snapshot to load board configuration from
	Snapshot *BoardSnapshot
	// Whether to set all cells as unrevealed when loading the Snapshot
	LoadSnapshotFresh bool

	Director         Director
	DirectorTickRate time.Duration

	// Transparency of annotations when first displayed
	AnnotationBaseAlpha float64
	// Total time an annotation will be displayed
	AnnotationDuration time.Duration

	// Path to directory where final snapshots of boards should be saved
	SavedSnapshotsDir string
}

func NewGameConfig() GameConfig {
	return GameConfig{
		Width:               30,
		Height:              16,
		NumMines:            99,
		Fullscreen:          false,
		MineDensity:         math.NaN(),
		Mode:                Classic,
		Director:            nil,
		DirectorTickRate:    25 * time.Millisecond,
		Snapshot:            nil,
		LoadSnapshotFresh:   true,
		AnnotationBaseAlpha: 0.5,
		AnnotationDuration:  200 * time.Millisecond,
	}
}

func (config GameConfig) createBoard() *Board {
	if config.Snapshot == nil {
		return createFilledBoard(boardConfig{
			Width:            config.Width,
			Height:           config.Height,
			NumMines:         config.NumMines,
			Mode:             config.Mode,
			Seed:             config.Seed,
			Director:         config.Director,
			DirectorTickRate: config.DirectorTickRate,
			OnGameEnd:        config.onGameEnd,
		})
	} else {
		return config.Snapshot.CreateBoard(
			boardConfig{
				Mode:             config.Mode,
				Director:         config.Director,
				DirectorTickRate: config.DirectorTickRate,
				OnGameEnd:        config.onGameEnd,
			},
			config.LoadSnapshotFresh,
		)
	}
}

func (config GameConfig) onGameEnd(board *Board) {
	config.saveSnapshot(board)
}

func (config GameConfig) saveSnapshot(board *Board) {
	if config.SavedSnapshotsDir != "" {
		stat, err := os.Stat(config.SavedSnapshotsDir)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(config.SavedSnapshotsDir, 0777); err != nil {
					fmt.Println(err)
					return
				}
			} else {
				fmt.Println(err)
				return
			}
		} else if !stat.Mode().IsDir() {
			fmt.Printf("%s is not a directory; cannot save snapshots to it.", config.SavedSnapshotsDir)
			return
		}

		filename := config.generateReplayFilename(board, time.Now())
		path := strings.Join([]string{config.SavedSnapshotsDir, filename}, string(os.PathSeparator))

		// TODO: prevent duplicate filenames
		file, err := os.Create(path)
		if err != nil {
			fmt.Println(err)
			return
		}

		snapshot := board.snapshot()
		if _, err := file.WriteString(snapshot.Serialize()); err != nil {
			fmt.Println(err)
			return
		}
	}
}

func (config GameConfig) generateReplayFilename(board *Board, t time.Time) string {
	filenameBuilder := strings.Builder{}

	filenameBuilder.WriteString(t.Format("20060102_150405_"))

	var stateStr string
	switch board.state {
	case Won:
		stateStr = "win"
	case Lost:
		stateStr = "loss"
	default:
		stateStr = "other"
	}
	filenameBuilder.WriteString(stateStr)

	filenameBuilder.WriteString(".yaml")

	return filenameBuilder.String()
}

func Run(config GameConfig) {
	headerHeight := uint(50)
	minWindowWith := float64(200)

	spritesheet := loadSpritesheet()
	windowIconImageData := spritesheet.Image().SubImage(image.Rectangle{
		Min: image.Point{Y: cellWidth * int(Flag+1)},
		Max: image.Point{X: cellWidth, Y: cellWidth * int(Flag+2)},
	})
	windowIcon := pixel.PictureDataFromImage(windowIconImageData)

	var monitor *pixelgl.Monitor = nil
	if config.Fullscreen {
		monitor = pixelgl.PrimaryMonitor()
	}

	cfg := pixelgl.WindowConfig{
		Title: "gosweep",
		Icon:  []pixel.Picture{windowIcon},
		Bounds: pixel.R(
			0, 0,
			math.Max(float64(config.Width*cellWidth), minWindowWith),
			float64(config.Height*cellWidth+headerHeight),
		),
		Monitor: monitor,
	}
	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	if config.Fullscreen {
		bounds := win.Bounds()
		config.Width = uint(bounds.W() / cellWidth)
		config.Height = uint((bounds.H() - float64(headerHeight)) / cellWidth)
	}

	if !math.IsNaN(config.MineDensity) {
		config.NumMines = uint(float64(config.Width*config.Height) * config.MineDensity)
	}

	batch := pixel.NewBatch(&pixel.TrianglesData{}, spritesheet)

	var boardTopLeft pixel.Vec

	basicAtlas := text.NewAtlas(basicfont.Face7x13, text.ASCII)
	var scoreText *text.Text
	var cellPosText *text.Text
	var hoveredCell *Cell

	var board *Board
	_resetBoard := func(paused bool) {
		batch.Clear()

		board = config.createBoard()
		if paused {
			board.TogglePaused()
		}
		board.startGame()

		win.SetBounds(
			pixel.R(
				0, 0,
				math.Max(float64(board.width*cellWidth), minWindowWith),
				float64(board.height*cellWidth+headerHeight),
			),
		)

		topLeft := win.Bounds().Vertices()[1]
		topRight := win.Bounds().Max
		boardTopLeft = topLeft.Sub(pixel.V(0, float64(headerHeight)))

		scoreText = text.New(topLeft.Add(pixel.V(20, -30)), basicAtlas)
		scoreText.Color = colornames.Black

		cellPosText = text.New(topRight.Add(pixel.V(-60, -30)), basicAtlas)
		cellPosText.Color = colornames.Darkcyan
	}
	resetBoard := func() {
		_resetBoard(false)
	}
	resetBoardPaused := func() {
		_resetBoard(true)
	}

	resetBoard()

	var (
		frames        = 0
		frameStart    = time.Now()
		frameDuration = time.Now().Sub(frameStart)
		frameDelay    = time.Second * 0
		tickRate      = time.Second / 120
		ticker        = time.NewTicker(tickRate)
		second        = time.Tick(time.Second)
	)

	requestFrame := func() {
		ticker.Reset(time.Nanosecond)
	}

	bgColor := colornames.Gainsboro
	for !win.Closed() {
		select {
		case <-second:
			win.SetTitle(fmt.Sprintf("%s | FPS: %d", cfg.Title, frames))
			frames = 0
		case <-ticker.C:
			frames++

			frameStart = time.Now()
			win.Clear(bgColor)

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

			for y, row := range board.cells {
				rowStart := boardTopLeft.Add(pixel.V(cellWidth/2, -float64(cellWidth/2+cellWidth*y)))

				for x, cell := range row {
					cellPos := rowStart.Add(pixel.V(float64(cellWidth*x), 0))

					if cell.isDirty {
						cell.sprite.Draw(batch, pixel.IM.Moved(cellPos))
						(&board.cells[y][x]).isDirty = false
					}
				}
			}
			batch.Draw(win)

			if board.directorAnnotations.Len() > 0 {
				imd := imdraw.New(nil)

				now := time.Now()
				for i := 0; i < board.directorAnnotations.Len(); i++ {
					annotation := board.directorAnnotations.At(i)

					timeShown := now.Sub(annotation.firstShown)
					isFromLatestFrame := annotation.frame == board.directorFrame

					if timeShown > config.AnnotationDuration && !isFromLatestFrame {
						board.directorAnnotations.PopFront()
						continue
					}

					cell := annotation.Cell
					start := boardTopLeft.Add(
						pixel.V(
							float64(cellWidth*cell.x),
							-float64(cellWidth*(cell.y+1)),
						),
					)
					end := start.Add(pixel.V(cellWidth, cellWidth))
					baseColor := pixel.Alpha(0)

					switch annotation.Type {
					case AnnotateClick:
						baseColor = pixel.RGB(1, 0, 0)
					case AnnotateRightClick:
						baseColor = pixel.RGB(0, 0, 1)
					case AnnotateMiddleClick:
						baseColor = pixel.RGB(0, 1, 0)
					case AnnotateHighlightYellow:
						baseColor = pixel.RGB(1, 1, 0)
					}

					alpha := config.AnnotationBaseAlpha
					if !isFromLatestFrame {
						progress := 1 - float64(timeShown)/float64(config.AnnotationDuration)
						alphaMultiplier := InOutCubic(progress)
						alpha *= alphaMultiplier
					}

					imd.Color = baseColor.Mul(pixel.Alpha(alpha))
					imd.Push(start, end)
					imd.Rectangle(0) // 0 = filled
				}

				imd.Draw(win)
			}
			win.Update()

			frameDuration = time.Now().Sub(frameStart)
			frameDelay = tickRate - frameDuration
			if frameDelay <= 0 {
				frameDelay = time.Nanosecond
			}
			ticker.Reset(frameDelay)
		default:
		}

		if board.canPlay() {
			// Pause with Space
			if win.JustPressed(pixelgl.KeySpace) {
				board.TogglePaused()
				requestFrame()
			}

			// Perform single step while paused with Right Arrow
			if board.state == Paused && (win.JustPressed(pixelgl.KeyRight) || win.Repeated(pixelgl.KeyRight)) {
				board.TogglePaused()
				board.RequestDirectorAct()
				board.TogglePaused()
				requestFrame()
			}
		} else {
			// Start a new game with Enter
			if win.JustPressed(pixelgl.KeyEnter) {
				config.Seed = board.rand.Int63()
				resetBoard()
				requestFrame()
			}

			// Start a new, paused game with Space or Right Arrow
			if win.JustPressed(pixelgl.KeySpace) || win.JustPressed(pixelgl.KeyRight) {
				config.Seed = board.rand.Int63()
				resetBoardPaused()
				requestFrame()
			}

			continue
		}

		if win.JustPressed(pixelgl.MouseButtonLeft) || win.JustPressed(pixelgl.MouseButtonRight) || win.JustPressed(pixelgl.MouseButtonMiddle) {
			if hoveredCell != nil {
				if win.JustPressed(pixelgl.MouseButtonLeft) {
					hoveredCell.click()
					requestFrame()
				}
				if win.JustPressed(pixelgl.MouseButtonRight) {
					hoveredCell.rightClick()
					requestFrame()
				}
				if win.JustPressed(pixelgl.MouseButtonMiddle) {
					hoveredCell.middleClick()
					requestFrame()
				}
			}
		}
	}
}

func InOutCubic(t float64) float64 {
	t *= 2
	if t < 1 {
		return 0.5 * t * t * t
	} else {
		t -= 2
		return 0.5 * (t*t*t + 2)
	}
}

func loadSpritesheet() *pixel.PictureData {
	img, _, err := image.Decode(bytes.NewReader(spritesheetPNG))
	if err != nil {
		panic(err)
	}
	spritesheet := pixel.PictureDataFromImage(img)

	x1, x2 := float64(0), float64(cellWidth)
	y2 := spritesheet.Bounds().Max.Y
	for _, state := range CellStates {
		frame := pixel.R(x1, y2-cellWidth, x2, y2)
		cellSprites[state] = pixel.NewSprite(spritesheet, frame)

		y2 -= cellWidth
	}

	return spritesheet
}
