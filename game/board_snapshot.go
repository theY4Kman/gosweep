package game

import (
	"gopkg.in/yaml.v2"
	"strings"
)

type BoardSnapshot struct {
	Seed            int64  `yaml:"seed"`
	Mode            string `yaml:"mode"`
	SerializedBoard string `yaml:"board,flow"`
}

var gameModes = map[string]GameMode{
	"win7":    Win7,
	"classic": Classic,
}

func (snapshot *BoardSnapshot) Serialize() string {
	out, err := yaml.Marshal(snapshot)
	if err != nil {
		panic(err)
	}

	return string(out)
}

func (snapshot *BoardSnapshot) CreateBoard(config boardConfig, fresh bool) *Board {
	rows := strings.Split(strings.TrimSpace(snapshot.SerializedBoard), "\n")

	config.Height = uint(len(rows))
	config.Width = uint(len(rows[0]))
	if config.Height == 0 || config.Width == 0 {
		return nil
	}

	config.Seed = snapshot.Seed
	config.Mode = gameModes[snapshot.Mode]
	config.NumMines = 0 // this will be calculated after mines are filled
	board := createBoard(config)

	mineCells := make(chan *Cell, config.Height*config.Width)

	for y, row := range rows {
		for x, c := range row {
			cell := board.CellAt(uint(x), uint(y))
			cell.deserialize(string(c), fresh)

			if cell.isMine {
				mineCells <- cell
			}
		}
	}
	close(mineCells)

	board.fillMines(mineCells)

	return board
}

func LoadSnapshot(in string) (*BoardSnapshot, error) {
	var snapshot BoardSnapshot
	if err := yaml.Unmarshal([]byte(in), &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}
