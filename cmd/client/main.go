package main

import (
	"context"
	"fmt"
	"log"

	pb "github.com/tomba07/bombshell/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	response, err := client.Join(
		context.Background(),
		&pb.JoinRequest{
			Name: "Alice",
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Joined as %s\n", response.PlayerId)
}
