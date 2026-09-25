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

	// symmetric interior walls
	for _, w := range [][2]int{
		// top-left L-cluster
		{5, 3}, {6, 3}, {7, 3}, {5, 4},
		// top-right L-cluster (mirrored)
		{22, 3}, {23, 3}, {24, 3}, {24, 4},
		// bottom-left L-cluster
		{5, 12}, {6, 12}, {7, 12}, {5, 11},
		// bottom-right L-cluster
		{22, 12}, {23, 12}, {24, 12}, {24, 11},
		// mid-left vertical wall (corridor chokepoint)
		{9, 4}, {9, 5}, {9, 6}, {9, 7},
		// mid-right vertical wall
		{20, 4}, {20, 5}, {20, 6}, {20, 7},
		// mid-left lower vertical wall
		{9, 9}, {9, 10}, {9, 11},
		// mid-right lower vertical wall
		{20, 9}, {20, 10}, {20, 11},
		// center block
		{14, 6}, {15, 6}, {14, 7}, {15, 7},
		{14, 8}, {15, 8}, {14, 9}, {15, 9},
		// top-center horizontal bars
		{11, 3}, {12, 3}, {13, 3},
		{16, 3}, {17, 3}, {18, 3},
		// bottom-center horizontal bars
		{11, 12}, {12, 12}, {13, 12},
		{16, 12}, {17, 12}, {18, 12},
	} {
		tiles[w[1]][w[0]] = tileWall
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
	best := position{}
	bestDist := -1

	for row := 1; row < g.height-1; row++ {
		for col := 1; col < g.width-1; col++ {
			if g.tiles[row][col] != tileEmpty {
				continue
			}
			candidate := position{col: col, row: row}

			occupied := false
			for _, p := range g.positions {
				if p == candidate {
					occupied = true
					break
				}
			}
			if occupied {
				continue
			}

			minDist := g.width * g.height
			for _, p := range g.positions {
				if d := manhattan(candidate, p); d < minDist {
					minDist = d
				}
			}
			if len(g.positions) == 0 {
				minDist = 0
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

func (g *gameState) detonate(t trap) (hit []string, respawns map[string]position, chained []trap) {
	delete(g.traps, t.id)
	tPos := position{col: t.col, row: t.row}

	for _, other := range g.traps {
		if inCross(tPos, position{col: other.col, row: other.row}, blastRadius) {
			chained = append(chained, other)
		}
	}

	respawns = make(map[string]position)
	for id, p := range g.positions {
		if inCross(tPos, p, blastRadius) {
			hit = append(hit, id)
		}
	}
	for _, id := range hit {
		delete(g.positions, id)
		spawn, ok := g.bestSpawn()
		if ok {
			g.positions[id] = spawn
			respawns[id] = spawn
		}
	}
	return
}

func inCross(center, p position, radius int) bool {
	sameCol := p.col == center.col && abs(p.row-center.row) <= radius
	sameRow := p.row == center.row && abs(p.col-center.col) <= radius
	return sameCol || sameRow
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
