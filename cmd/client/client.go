package main

import (
	"context"
	"fmt"

	pb "github.com/tomba07/bombshell/proto"
)

type Client struct {
	grpcClient pb.ChatServiceClient
	playerID   string
	players    map[string]position
}

type position struct {
	x int32
	y int32
}

func (c *Client) render() {
	const width = 10
	const height = 10

	// clear screen + move cursor to top-left
	fmt.Print("\033[2J\033[H")

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {

			if x == 0 || x == width-1 ||
				y == 0 || y == height-1 {
				fmt.Print("#")
				continue
			}

			playerHere := false

			for playerID, pos := range c.players {
				if pos.x == int32(x) && pos.y == int32(y) {
					if playerID == c.playerID {
						fmt.Print("@")
					} else {
						fmt.Print("P")
					}

					playerHere = true
					break
				}
			}

			if !playerHere {
				fmt.Print(".")
			}
		}

		fmt.Print("\r\n")
	}

	fmt.Print("\r\nArrow keys to move, q to quit\r\n")
}

func (c *Client) handleEvent(event *pb.Event) {
	switch event.Type {
	case pb.EventType_EVENT_TYPE_PLAYER_JOINED:
		c.players[event.PlayerId] = position{
			x: event.X,
			y: event.Y,
		}

	case pb.EventType_EVENT_TYPE_PLAYER_LEFT:
		delete(c.players, event.PlayerId)

	case pb.EventType_EVENT_TYPE_MOVE:
		c.players[event.PlayerId] = position{
			x: event.X,
			y: event.Y,
		}

	case pb.EventType_EVENT_TYPE_CHAT:
		// ignore for rendering for now
	}

	c.render()
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
