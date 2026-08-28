package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	pb "github.com/tomba07/bombshell/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func leaveClient(client *Client) {
	if err := client.Leave(); err != nil {
		log.Printf("failed to leave: %v\n", err)
	}
}

func handleShutdown(client *Client) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		leaveClient(client)
		os.Exit(0)
	}()
}

func readName(scanner *bufio.Scanner) string {
	for {
		fmt.Print("Enter your name: ")

		if !scanner.Scan() {
			check(scanner.Err())
			log.Fatal("no name entered")
		}

		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			return name
		}

		fmt.Println("Name cannot be empty.")
	}
}

func main() {
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	check(err)
	defer conn.Close()

	client := &Client{
		grpcClient: pb.NewChatServiceClient(conn),
	}

	scanner := bufio.NewScanner(os.Stdin)

	name := readName(scanner)
	check(client.Join(name))

	go func() {
		if err := client.Subscribe(); err != nil {
			log.Printf("subscribe ended %v\n", err)
		}
	}()
	fmt.Printf("Joined as %s\n", client.playerID)
	defer leaveClient(client)

	handleShutdown(client)

	fmt.Println("Type a message and press Enter:")

	for scanner.Scan() {
		check(client.SendMessage(scanner.Text()))
	}

	check(scanner.Err())
}
