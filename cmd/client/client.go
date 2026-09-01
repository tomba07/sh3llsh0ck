package main

import (
	"context"
	"fmt"

	pb "github.com/tomba07/bombshell/proto"
)

type Client struct {
	grpcClient pb.ChatServiceClient
	playerID   string
}

func (c *Client) handleEvent(event *pb.Event) {
	switch event.Type {
	case pb.EventType_EVENT_TYPE_CHAT:
		fmt.Printf("[%s] %s\n", event.PlayerId, event.Message)
	case pb.EventType_EVENT_TYPE_PLAYER_JOINED:
		fmt.Println(event.Message)
	case pb.EventType_EVENT_TYPE_PLAYER_LEFT:
		fmt.Println(event.Message)
	case pb.EventType_EVENT_TYPE_MOVE:
		fmt.Printf("%s moved %v\n", event.PlayerId, event.Direction)
	default:
		fmt.Printf("unknown event: %v\n", event.Type)
	}
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
