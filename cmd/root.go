
package cmd

import (
	"fmt"
	"github.com/faiface/pixel/pixelgl"
	"github.com/spf13/cobra"
	"github.com/they4kman/gosweep/director/constraint"
	"github.com/they4kman/gosweep/game"
	"math/rand"
	"os"
	"time"
)

var gameConfig = game.NewGameConfig()
var useDirector = false

var rootCmd = &cobra.Command{
	Use:   "gosweep",
	Short: "Play manual or computer-driven Minesweeper",
	Long: `gosweep is a Minesweeper game which supports human- or
computer-driven playing.

Run with no arguments to play manually
	gosweep

Use the director flag to make the computer play for you
	gosweep -director
`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: allow customization of seed by command-line
		rand.Seed(time.Now().UnixNano())

		if useDirector {
			gameConfig.Director = &constraint.Director{}
		}

		pixelgl.Run(func() {
			game.Run(gameConfig)
		})
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type gameModeValue game.GameMode

func newGameModeValue(val game.GameMode, p *game.GameMode) *gameModeValue {
	*p = val
	return (*gameModeValue)(p)
}

var gameModes = map[string]game.GameMode{
	"win7": game.Win7,
	"classic": game.Classic,
}

func (modeVal *gameModeValue) String() string {
	for name, mode := range gameModes {
		if mode == game.GameMode(*modeVal) {
			return name
		}
	}
	return fmt.Sprint(*modeVal)
}

func (modeVal *gameModeValue) Set(value string) error {
	if mode, isValid := gameModes[value]; isValid {
		*modeVal = gameModeValue(mode)
		return nil
	} else {
		return fmt.Errorf("invalid game mode")
	}
}

func (modeVal *gameModeValue) Type() string {
	return "game.GameMode"
}

func init() {
	// Define our root -help without a shorthand, as we'll use -h for --height
	// Ref: https://github.com/spf13/cobra/issues/291
	rootCmd.Flags().Bool("help", false, "Help for this command")

	rootCmd.Flags().UintVarP(&gameConfig.Width, "width", "w", 30, "Width of game board, in cells")
	rootCmd.Flags().UintVarP(&gameConfig.Height, "height", "h", 16, "Height of game board, in cells")
	rootCmd.Flags().UintVarP(&gameConfig.NumMines, "mines", "m", 99, "Number of mines to place in the game board")
	rootCmd.Flags().Var(newGameModeValue(game.Win7, &gameConfig.Mode), "mode", `Game mode, controlling behaviour of first click.
win7: all cells surrounding the first-clicked cell are cleared of mines (first click never loses)
classic: mines are left as is (first click can lose the game)`)
	rootCmd.Flags().BoolVarP(&useDirector, "director", "d", false, "Make the computer play")
}
