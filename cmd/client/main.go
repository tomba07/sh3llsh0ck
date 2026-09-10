package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	pb "github.com/tomba07/bombshell/proto"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	check(err)
	defer conn.Close()

	client := &Client{
		grpcClient: pb.NewChatServiceClient(conn),
		players:    make(map[string]position),
		traps:      make(map[string]trap),
		blasts:     make(map[string][]position),
	}

	scanner := bufio.NewScanner(os.Stdin)

	name := readName(scanner)
	check(client.Join(name))
	defer leaveClient(client)

	fmt.Printf("Joined as %s\n", client.playerID)
	fmt.Println("Use arrow keys to move, q to quit")

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	check(err)
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	go func() {
		if err := client.Subscribe(); err != nil {
			log.Printf("subscribe ended: %v\r\n", err)
		}
	}()

	fmt.Print("\033[2J\033[H")

	buffer := make([]byte, 1)

	for {
		_, err := os.Stdin.Read(buffer)
		check(err)

		switch buffer[0] {
		case 'q':
			return

		case ' ':
			check(client.PlaceTrap())

		case 27: // ESC
			sequence := make([]byte, 2)

			_, err := os.Stdin.Read(sequence)
			check(err)

			if sequence[0] != '[' {
				continue
			}

			switch sequence[1] {
			case 'A':
				check(client.Move(pb.Direction_DIRECTION_UP))

			case 'B':
				check(client.Move(pb.Direction_DIRECTION_DOWN))

			case 'C':
				check(client.Move(pb.Direction_DIRECTION_RIGHT))

			case 'D':
				check(client.Move(pb.Direction_DIRECTION_LEFT))
			}
		}
	}
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

func leaveClient(client *Client) {
	if err := client.Leave(); err != nil {
		log.Printf("failed to leave: %v\n", err)
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
