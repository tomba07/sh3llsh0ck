package main

import (
	"context"
	"fmt"
	"time"

	pb "github.com/tomba07/bombshell/proto"
)

type Client struct {
	grpcClient pb.ChatServiceClient
	playerID   string
	width      int32
	height     int32
	players    map[string]position
	traps      map[string]position
	blasts     map[string][]position
}

type position struct {
	col int32
	row int32
}

func (c *Client) Subscribe() error {
	stream, err := c.grpcClient.Subscribe(
		context.Background(),
		&pb.SubscribeRequest{
			PlayerId: c.playerID,
		},
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

func (c *Client) handleEvent(event *pb.Event) {
	switch event.Type {
	case pb.EventType_EVENT_TYPE_PLAYER_JOINED:
		c.players[event.PlayerId] = position{
			col: event.Col,
			row: event.Row,
		}

	case pb.EventType_EVENT_TYPE_PLAYER_LEFT:
		delete(c.players, event.PlayerId)

	case pb.EventType_EVENT_TYPE_MOVE:
		c.players[event.PlayerId] = position{
			col: event.Col,
			row: event.Row,
		}

	case pb.EventType_EVENT_TYPE_CHAT:
		// ignore for rendering for now

	case pb.EventType_EVENT_TYPE_TRAP_PLACED:
		c.traps[event.PlayerId] = position{col: event.Col, row: event.Row}

	case pb.EventType_EVENT_TYPE_TRAP_TRIGGERED:
		var tiles []position
		radius := int32(event.BlastRadius)
		tiles = append(tiles, position{col: event.Col, row: event.Row})
		for i := int32(1); i <= radius; i++ {
			tiles = append(tiles,
				position{col: event.Col + i, row: event.Row},
				position{col: event.Col - i, row: event.Row},
				position{col: event.Col, row: event.Row + i},
				position{col: event.Col, row: event.Row - i},
			)
		}
		delete(c.traps, event.PlayerId)
		c.blasts[event.PlayerId] = tiles
		c.render()
		go func() {
			time.Sleep(1000 * time.Millisecond)
			delete(c.blasts, event.PlayerId)
			c.render()
		}()
		return // skip the render() at the bottom — already called
	}

	c.render()
}

const (
	colorReset         = "\033[0m"
	colorDarkGray      = "\033[90m"
	colorBrightGreen   = "\033[92m"
	colorBrightYellow  = "\033[93m"
	colorBrightRed     = "\033[91m"
	colorBrightMagenta = "\033[95m"
)

func (c *Client) render() {
	// move cursor to top-left
	fmt.Print("\033[H")

	for row := int32(0); row < c.height; row++ {
		for col := int32(0); col < c.width; col++ {

			if col == 0 || col == c.width-1 ||
				row == 0 || row == c.height-1 {
				fmt.Print(colorDarkGray + "#" + colorReset)
				continue
			}

			trapHere := false
			for _, t := range c.traps {
				if t.col == col && t.row == row {
					fmt.Print(colorBrightMagenta + "*" + colorReset)
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

		fmt.Print("\r\n")
	}

	fmt.Print("\r\nArrow keys to move, q to quit\r\n")
}

func (c *Client) Join(name string) error {
	response, err := c.grpcClient.Join(
		context.Background(),
		&pb.JoinRequest{
			Name: name,
		},
	)
	if err != nil {
		return err
	}

	c.playerID = response.PlayerId
	c.width = response.Width
	c.height = response.Height
	return nil
}

func (c *Client) Move(direction pb.Direction) error {
	_, err := c.grpcClient.Move(
		context.Background(),
		&pb.MoveRequest{
			PlayerId:  c.playerID,
			Direction: direction,
		},
	)

	return err
}

func (c *Client) PlaceTrap() error {
	_, err := c.grpcClient.PlaceTrap(
		context.Background(),
		&pb.PlaceTrapRequest{PlayerId: c.playerID},
	)
	return err
}

func (c *Client) SendMessage(message string) error {
	_, err := c.grpcClient.SendMessage(
		context.Background(),
		&pb.SendMessageRequest{
			PlayerId: c.playerID,
			Message:  message,
		},
	)

	return err
}

func (c *Client) Leave() error {
	_, err := c.grpcClient.Leave(
		context.Background(),
		&pb.LeaveRequest{
			PlayerId: c.playerID,
		},
	)

	return err
}
