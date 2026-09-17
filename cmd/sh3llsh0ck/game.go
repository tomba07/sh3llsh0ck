package main

type tile int

const (
	tileEmpty tile = iota
	tileWall
)

const blastRadius = 2

type position struct {
	col int
	row int
}

type gameState struct {
	width     int
	height    int
	tiles     [][]tile
	positions map[string]position
	traps     map[string]trap
}

const maxTrapsPerPlayer = 3

type trap struct {
	id      string
	col     int
	row     int
	ownerID string
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
		traps:     make(map[string]trap),
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

			minDist := g.width * g.height

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

func (g *gameState) placeTrap(playerID, trapID string) (trap, bool) {
	pos, ok := g.positions[playerID]
	if !ok {
		return trap{}, false
	}

	count := 0
	for _, t := range g.traps {
		if t.col == pos.col && t.row == pos.row {
			return trap{}, false
		}
		if t.ownerID == playerID {
			count++
		}
	}
	if count >= maxTrapsPerPlayer {
		return trap{}, false
	}

	t := trap{id: trapID, col: pos.col, row: pos.row, ownerID: playerID}
	g.traps[trapID] = t

	return t, true
}

func (g *gameState) detonate(t trap) (hit []string, respawns map[string]position) {
	delete(g.traps, t.id)

	respawns = make(map[string]position)
	for id, p := range g.positions {
		sameCol := p.col == t.col && abs(p.row-t.row) <= blastRadius
		sameRow := p.row == t.row && abs(p.col-t.col) <= blastRadius
		if sameCol || sameRow {
			hit = append(hit, id)
		}
	}
	for _, id := range hit {
		spawn, ok := g.bestSpawn()
		if ok {
			g.positions[id] = spawn
			respawns[id] = spawn
		}
	}
	return
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
