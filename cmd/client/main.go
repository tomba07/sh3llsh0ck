package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	pb "github.com/tomba07/bombshell/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	check(err)
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	response, err := client.Join(
		context.Background(),
		&pb.JoinRequest{
			Name: "Alice",
		},
	)
	check(err)

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Printf("Joined as %s\n", response.PlayerId)
	fmt.Println("Type a message and press Enter:")

	for scanner.Scan() {
		message := scanner.Text()

		_, err = client.SendMessage(
			context.Background(),
			&pb.SendMessageRequest{
				PlayerId: response.PlayerId,
				Message:  message,
			},
		)
		check(err)
	}
	check(scanner.Err())
}
