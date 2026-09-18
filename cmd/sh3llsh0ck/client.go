package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	pb "github.com/tomba07/bombshell/proto"
)

type trapBlink struct {
	cancel context.CancelFunc
}

type gameClientView struct {
	mu         sync.Mutex
	grpcClient pb.ChatServiceClient
	playerID   string
	width      int
	height     int
	players    map[string]position
	traps      map[string]trapView
	blasts     map[string][]position
	scores     map[string]int
	trapBlinks map[string]trapBlink
}

type trapView struct {
	pos   position
	color string
}

const (
	colorReset         = "\033[0m"
	colorDarkGray      = "\033[90m"
	colorBrightGreen   = "\033[92m"
	colorBrightYellow  = "\033[93m"
	colorBrightRed     = "\033[91m"
	colorBrightMagenta = "\033[95m"
)

func (c *gameClientView) Subscribe() error {
	stream, err := c.grpcClient.Subscribe(
		context.Background(),
		&pb.SubscribeRequest{PlayerId: c.playerID},
	)
	if err != nil {
		return err
	}

	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}
		c.handleEvent(event)
	}
}

func (c *gameClientView) handleEvent(event *pb.Event) {
	c.mu.Lock()
	switch event.Type {
	case pb.EventType_EVENT_TYPE_PLAYER_JOINED:
		c.players[event.PlayerId] = position{col: int(event.Col), row: int(event.Row)}

	case pb.EventType_EVENT_TYPE_PLAYER_LEFT:
		delete(c.players, event.PlayerId)

	case pb.EventType_EVENT_TYPE_MOVE:
		c.players[event.PlayerId] = position{col: int(event.Col), row: int(event.Row)}

	case pb.EventType_EVENT_TYPE_CHAT:
		// ignore for rendering

	case pb.EventType_EVENT_TYPE_TRAP_PLACED:
		trapID := event.TrapId
		if b, ok := c.trapBlinks[trapID]; ok {
			b.cancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		c.trapBlinks[trapID] = trapBlink{cancel: cancel}
		c.traps[trapID] = trapView{
			pos:   position{col: int(event.Col), row: int(event.Row)},
			color: colorBrightMagenta,
		}
		c.mu.Unlock()
		go func(tid string) {
			blink := func(d time.Duration) bool {
				time.Sleep(d)
				c.mu.Lock()
				if ctx.Err() != nil {
					c.mu.Unlock()
					return false
				}
				t := c.traps[tid]
				t.color = colorBrightYellow
				c.traps[tid] = t
				c.render()
				c.mu.Unlock()

				time.Sleep(d)
				c.mu.Lock()
				if ctx.Err() != nil {
					c.mu.Unlock()
					return false
				}
				t = c.traps[tid]
				t.color = colorBrightMagenta
				c.traps[tid] = t
				c.render()
				c.mu.Unlock()
				return true
			}
			for range 3 {
				if !blink(200 * time.Millisecond) {
					return
				}
			}
			for range 5 {
				if !blink(60 * time.Millisecond) {
					return
				}
			}
		}(trapID)
		return

	case pb.EventType_EVENT_TYPE_SCORE_UPDATE:
		c.scores[event.PlayerId] = int(event.Score)

	case pb.EventType_EVENT_TYPE_TRAP_TRIGGERED:
		radius := int(event.BlastRadius)
		blastKey := fmt.Sprintf("chain-%d", time.Now().UnixNano())
		var allTiles []position
		for _, b := range event.Blasts {
			if blink, ok := c.trapBlinks[b.TrapId]; ok {
				blink.cancel()
				delete(c.trapBlinks, b.TrapId)
			}
			delete(c.traps, b.TrapId)
			col, row := int(b.Col), int(b.Row)
			allTiles = append(allTiles, position{col: col, row: row})
			for i := 1; i <= radius; i++ {
				allTiles = append(allTiles,
					position{col: col + i, row: row},
					position{col: col - i, row: row},
					position{col: col, row: row + i},
					position{col: col, row: row - i},
				)
			}
		}
		c.blasts[blastKey] = allTiles
		c.render()
		c.mu.Unlock()
		go func() {
			time.Sleep(1000 * time.Millisecond)
			c.mu.Lock()
			delete(c.blasts, blastKey)
			c.render()
			c.mu.Unlock()
		}()
		return
	}

	c.render()
	c.mu.Unlock()
}

func (c *gameClientView) render() {
	top := c.topScorers(3)
	boardWidth := c.width // one char per cell

	fmt.Print("\033[H")

	for row := 0; row < c.height; row++ {
		for col := 0; col < c.width; col++ {
			if col == 0 || col == c.width-1 || row == 0 || row == c.height-1 {
				fmt.Print(colorDarkGray + "#" + colorReset)
				continue
			}

			trapHere := false
			for _, t := range c.traps {
				if t.pos.col == col && t.pos.row == row {
					fmt.Print(t.color + "*" + colorReset)
					trapHere = true
					break
				}
			}
			if trapHere {
				continue
			}

			blastHere := false
			for _, tiles := range c.blasts {
				for _, t := range tiles {
					if t.col == col && t.row == row {
						fmt.Print(colorBrightRed + "X" + colorReset)
						blastHere = true
						break
					}
				}
				if blastHere {
					break
				}
			}
			if blastHere {
				continue
			}

			playerHere := false
			for playerID, pos := range c.players {
				if pos.col == col && pos.row == row {
					if playerID == c.playerID {
						fmt.Print(colorBrightGreen + "@" + colorReset)
					} else {
						fmt.Print(colorBrightYellow + "P" + colorReset)
					}
					playerHere = true
					break
				}
			}

			if !playerHere {
				fmt.Print(colorDarkGray + "." + colorReset)
			}
		}

		// leaderboard column to the right
		sideCol := boardWidth + 2
		switch row {
		case 0:
			fmt.Printf("\033[%d;%dH%s", row+1, sideCol, colorBrightGreen+"Top Players"+colorReset)
		case 1:
			fmt.Printf("\033[%d;%dH%s", row+1, sideCol, colorDarkGray+"───────────"+colorReset)
		default:
			i := row - 2
			if i < len(top) {
				name, score := top[i].name, top[i].score
				medal := " "
				switch i {
				case 0:
					medal = colorBrightYellow + "1" + colorReset
				case 1:
					medal = colorDarkGray + "2" + colorReset
				case 2:
					medal = colorBrightRed + "3" + colorReset
				}
				fmt.Printf("\033[%d;%dH%s %-8s %d", row+1, sideCol, medal, name, score)
			}
		}

		fmt.Print("\r\n")
	}

	fmt.Print("\r\nArrow keys to move, space to place trap, q to quit")
}

type scorerEntry struct {
	name  string
	score int
}

func (c *gameClientView) topScorers(n int) []scorerEntry {
	entries := make([]scorerEntry, 0, len(c.scores))
	for name, score := range c.scores {
		entries = append(entries, scorerEntry{name, score})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score > entries[j].score
	})
	if len(entries) > n {
		entries = entries[:n]
	}
	return entries
}

func (c *gameClientView) Join(name string) error {
	response, err := c.grpcClient.Join(
		context.Background(),
		&pb.JoinRequest{Name: name},
	)
	if err != nil {
		return err
	}

	c.playerID = response.PlayerId
	c.width = int(response.Width)
	c.height = int(response.Height)
	return nil
}

func (c *gameClientView) Move(direction pb.Direction) error {
	_, err := c.grpcClient.Move(
		context.Background(),
		&pb.MoveRequest{PlayerId: c.playerID, Direction: direction},
	)
	return err
}

func (c *gameClientView) PlaceTrap() error {
	_, err := c.grpcClient.PlaceTrap(
		context.Background(),
		&pb.PlaceTrapRequest{PlayerId: c.playerID},
	)
	return err
}

func (c *gameClientView) Leave() error {
	_, err := c.grpcClient.Leave(
		context.Background(),
		&pb.LeaveRequest{PlayerId: c.playerID},
	)
	return err
}
