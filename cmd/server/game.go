package main

type tile int

const (
	tileEmpty tile = iota
	tileWall
)

type position struct {
	col int
	row int
}

type gameState struct {
	width     int
	height    int
	tiles     [][]tile
	positions map[string]position
}

func newGameState(width, height int) gameState {
	tiles := make([][]tile, height)

	for row := range height {
		tiles[row] = make([]tile, width)
	}

	for col := range width {
		tiles[0][col] = tileWall
		tiles[height-1][col] = tileWall
	}

	for row := range height {
		tiles[row][0] = tileWall
		tiles[row][width-1] = tileWall
	}

	return gameState{
		width:     width,
		height:    height,
		tiles:     tiles,
		positions: make(map[string]position),
	}
}

func (g *gameState) bestSpawn() (position, bool) {
	if len(g.positions) == 0 {
		return position{col: g.width / 2, row: g.height / 2}, true
	}

	best := position{}
	bestDist := -1

	for row := 1; row < g.height-1; row++ {
		for col := 1; col < g.width-1; col++ {
			if g.tiles[row][col] != tileEmpty {
				continue
			}
			candidate := position{col: col, row: row}
			alreadyOccupied := false

			for _, p := range g.positions {
				if p == candidate {
					alreadyOccupied = true
					break
				}
			}
			if alreadyOccupied {
				continue
			}

			minDist := g.width * g.height // large sentinel

			for _, p := range g.positions {
				if d := manhattan(candidate, p); d < minDist {
					minDist = d
				}
			}
			if minDist > bestDist {
				bestDist = minDist
				best = candidate
			}
		}
	}

	return best, bestDist != -1
}

func manhattan(a, b position) int {
	return abs(a.col-b.col) + abs(a.row-b.row)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
