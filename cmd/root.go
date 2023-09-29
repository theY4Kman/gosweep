package cmd

import (
	"fmt"
	"github.com/faiface/pixel/pixelgl"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/they4kman/gosweep/director/constraint"
	"github.com/they4kman/gosweep/game"
	"io"
	"os"
	"time"
)

var gameConfig = game.NewGameConfig()
var useDirector = false
var savedSnapshotsDir string
var snapshotToLoad string
var verbosity string

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
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if useDirector {
			gameConfig.Director = &constraint.Director{}
		}

		if !cmd.Flag("seed").Changed {
			gameConfig.Seed = time.Now().UnixNano()
		}

		if savedSnapshotsDir != "" {
			stat, err := os.Stat(savedSnapshotsDir)
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else if !stat.Mode().IsDir() {
				return fmt.Errorf("%s is not a directory; cannot save snapshots to it", savedSnapshotsDir)
			}

			gameConfig.SavedSnapshotsDir = savedSnapshotsDir
		}

		if snapshotToLoad != "" {
			stat, err := os.Stat(snapshotToLoad)
			if err != nil {
				return err
			} else if !stat.Mode().IsRegular() {
				return fmt.Errorf("%s is not a valid file", snapshotToLoad)
			}

			file, err := os.Open(snapshotToLoad)
			if err != nil {
				return err
			}

			bytes, err := io.ReadAll(file)
			if err != nil {
				return err
			}

			var snapshot *game.BoardSnapshot
			if snapshot, err = game.LoadSnapshot(string(bytes)); err != nil {
				return err
			}

			gameConfig.Snapshot = snapshot
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
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
	"win7":    game.Win7,
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
	return "game mode"
}

func setUpLogging(out io.Writer, level string) error {
	logrus.SetOutput(out)
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)
	return nil
}

func init() {
	// Define our root -help without a shorthand, as we'll use -h for --height
	// Ref: https://github.com/spf13/cobra/issues/291
	rootCmd.Flags().Bool("help", false, "Help for this command")

	rootCmd.Flags().UintVarP(&gameConfig.Width, "width", "w", gameConfig.Width, "Width of game board, in cells")
	rootCmd.Flags().UintVarP(&gameConfig.Height, "height", "h", gameConfig.Height, "Height of game board, in cells")
	rootCmd.Flags().UintVarP(&gameConfig.NumMines, "mines", "m", gameConfig.NumMines, "Number of mines to place in the game board")
	rootCmd.Flags().BoolVar(&gameConfig.Fullscreen, "fullscreen", gameConfig.Fullscreen, "Whether to run in fullscreen mode (overrides --width and --height)")
	rootCmd.Flags().Float64Var(&gameConfig.MineDensity, "mine-density", gameConfig.MineDensity, "Percentage of mines to cells in the board (overrides --mines)")
	rootCmd.Flags().Var(newGameModeValue(game.Win7, &gameConfig.Mode), "mode", `Game mode, controlling behaviour of first click.
 - win7:    all cells surrounding the first-clicked cell are cleared of mines
            (first click never loses)
 - classic: mines are left as is
            (first click can lose the game)`)
	rootCmd.Flags().BoolVarP(&useDirector, "director", "d", false, "Make the computer play")
	rootCmd.Flags().DurationVar(&gameConfig.DirectorTickRate, "tick-rate", gameConfig.DirectorTickRate, "Make the computer play")
	rootCmd.Flags().Int64Var(&gameConfig.Seed, "seed", 1, "Initial seed to feed into random number generator")

	rootCmd.Flags().StringVar(&savedSnapshotsDir, "save-snapshots-to", "", "Directory to save endgame board snapshots to")
	rootCmd.Flags().StringVar(&snapshotToLoad, "load", "", "Board snapshot to load and play")
	rootCmd.Flags().BoolVar(&gameConfig.LoadSnapshotFresh, "load-fresh", gameConfig.LoadSnapshotFresh, "Whether to load the specified snapshot completely unrevealed")

	rootCmd.PersistentFlags().StringVarP(&verbosity, "verbosity", "v", logrus.WarnLevel.String(), "Log level (debug, info, warn, error, fatal, panic")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := setUpLogging(os.Stdout, verbosity); err != nil {
			return err
		}
		return nil
	}
}
