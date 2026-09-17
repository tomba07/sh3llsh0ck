package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/tomba07/bombshell/proto"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	choice := selectOption("sh3llsh0ck", []string{"Host a game", "Join a game"})

	var addr string
	if choice == 0 {
		go startServer(50051)
		time.Sleep(100 * time.Millisecond)
		addr = "localhost:50051"
	} else {
		addr = readLine("Server address [localhost:50051]: ")
		if addr == "" {
			addr = "localhost:50051"
		}
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	check(err)
	defer conn.Close()

	client := &gameClientView{
		grpcClient: pb.NewChatServiceClient(conn),
		players:    make(map[string]position),
		traps:      make(map[string]trapView),
		blasts:     make(map[string][]position),
		scores:     make(map[string]int),
	}

	name := readLine("Enter your name: ")
	for name == "" {
		name = readLine("Name cannot be empty. Enter your name: ")
	}

	check(client.Join(name))
	defer func() {
		if err := client.Leave(); err != nil {
			log.Printf("failed to leave: %v\n", err)
		}
	}()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	check(err)
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Print("\033[?1049h\033[2J\033[H") // enter alternate screen
	defer fmt.Print("\033[?1049l")        // exit alternate screen

	go func() {
		if err := client.Subscribe(); err != nil {
			log.Printf("subscribe ended: %v\r\n", err)
		}
	}()

	buf := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buf)
		check(err)

		switch buf[0] {
		case 'q':
			return
		case ' ':
			check(client.PlaceTrap())
		case 27:
			seq := make([]byte, 2)
			_, err := os.Stdin.Read(seq)
			check(err)
			if seq[0] != '[' {
				continue
			}
			switch seq[1] {
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

func readLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
