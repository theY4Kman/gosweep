package game

import (
	"gopkg.in/yaml.v2"
	"strings"
	"sync"
)

type BoardSnapshot struct {
	Seed            int64  `yaml:"seed"`
	SerializedBoard string `yaml:"board,flow"`
}

func (snapshot *BoardSnapshot) Serialize() string {
	out, err := yaml.Marshal(snapshot)
	if err != nil {
		panic(err)
	}

	return string(out)
}

func (snapshot *BoardSnapshot) CreateBoard(config boardConfig, fresh bool) *Board {
	rows := strings.Split(snapshot.SerializedBoard, "\n")

	config.Height = uint(len(rows))
	config.Width = uint(len(rows[0]))
	if config.Height == 0 || config.Width == 0 {
		return nil
	}

	config.Seed = snapshot.Seed
	board := createBoard(config)

	mineCells := make(chan *Cell)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for y, row := range rows {
			for x, c := range row {
				cell := board.CellAt(uint(x), uint(y))
				cell.deserialize(string(c))

				if fresh {
					cell.isFlagged = false
					cell.isRevealed = false
					cell.setState(Unrevealed)
				}

				if cell.isMine {
					mineCells <- cell
				}
			}
		}
		close(mineCells)

		wg.Done()
	}()

	board.fillMines(mineCells)
	wg.Wait()

	return board
}

func LoadSnapshot(in string) (*BoardSnapshot, error) {
	var snapshot BoardSnapshot
	if err := yaml.Unmarshal([]byte(in), &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}
