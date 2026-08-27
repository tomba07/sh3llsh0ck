package main

import (
	"context"

	pb "github.com/tomba07/bombshell/proto"
)

type Client struct {
	grpcClient pb.ChatServiceClient
	playerID   string
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
