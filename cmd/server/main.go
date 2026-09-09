package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	pb "github.com/tomba07/bombshell/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedChatServiceServer

	mu      sync.Mutex
	clients map[string]*client
	game    gameState
}

type client struct {
	name     string
	messages chan *pb.Event
}

type tile int

const (
	tileEmpty tile = iota
	tileWall
)

type position struct {
	col int
	row int
}

type gameState struct {
	width     int
	height    int
	tiles     [][]tile
	positions map[string]position
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func manhattan(a, b position) int {
	return abs(a.col-b.col) + abs(a.row-b.row)
}

func (g *gameState) bestSpawn() (position, bool) {
	best := position{}
	bestDist := -1

	// First player spawns in center
	if len(g.positions) == 0 {
		return position{col: g.width / 2, row: g.height / 2}, true
	}

	for row := 1; row < g.height-1; row++ {
		for col := 1; col < g.width-1; col++ {
			if g.tiles[row][col] != tileEmpty {
				continue
			}
			candidate := position{col: col, row: row}
			alreadyOccupied := false

			for _, p := range g.positions {
				if p == candidate {
					alreadyOccupied = true
					break
				}
			}
			if alreadyOccupied {
				continue
			}

			minDist := g.width * g.height // large sentinel

			for _, p := range g.positions {
				if d := manhattan(candidate, p); d < minDist {
					minDist = d
				}
			}
			if minDist > bestDist {
				bestDist = minDist
				best = candidate
			}
		}
	}

	return best, bestDist != -1
}

func newGameState(width, height int) gameState {
	tiles := make([][]tile, height)

	for row := range height {
		tiles[row] = make([]tile, width)
	}

	for col := range width {
		tiles[0][col] = tileWall
		tiles[height-1][col] = tileWall
	}

	for row := range height {
		tiles[row][0] = tileWall
		tiles[row][width-1] = tileWall
	}

	return gameState{
		width:     width,
		height:    height,
		tiles:     tiles,
		positions: make(map[string]position),
	}
}

func (s *server) getClient(playerID string) (*client, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, joined := s.clients[playerID]
	return client, joined
}

func (s *server) snapshotClients() []*client {
	s.mu.Lock()
	defer s.mu.Unlock()

	clients := make([]*client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}

	return clients
}

func broadcast(clients []*client, event *pb.Event) {
	for _, client := range clients {
		client.messages <- event
	}
}

func (s *server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	s.mu.Lock()

	if _, exists := s.clients[req.Name]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("name %q already taken", req.Name)
	}

	s.clients[req.Name] = &client{
		name:     req.Name,
		messages: make(chan *pb.Event, 10),
	}

	clients := make([]*client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}

	best, ok := s.game.bestSpawn()
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("no available spawn positions")
	}

	// Add existing players
	for id, pos := range s.game.positions {
		s.clients[req.Name].messages <- &pb.Event{
			Type:     pb.EventType_EVENT_TYPE_PLAYER_JOINED,
			PlayerId: id,
			X:        int32(pos.col),
			Y:        int32(pos.row),
		}
	}

	s.game.positions[req.Name] = best

	s.mu.Unlock()

	message := fmt.Sprintf("%s joined", req.Name)
	pos := s.game.positions[req.Name]
	event := &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_PLAYER_JOINED,
		PlayerId: req.Name,
		Message:  message,
		X:        int32(pos.col),
		Y:        int32(pos.row),
	}

	broadcast(clients, event)

	fmt.Printf("%s joined\r\n", req.Name)

	return &pb.JoinResponse{
		PlayerId: req.Name,
		Width:    int32(s.game.width),
		Height:   int32(s.game.height),
	}, nil
}

func (s *server) Move(
	ctx context.Context,
	req *pb.MoveRequest,
) (*pb.MoveResponse, error) {
	s.mu.Lock()

	pos, joined := s.game.positions[req.PlayerId]
	if !joined {
		s.mu.Unlock()
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	switch req.Direction {
	case pb.Direction_DIRECTION_UP:
		pos.row--
	case pb.Direction_DIRECTION_DOWN:
		pos.row++
	case pb.Direction_DIRECTION_LEFT:
		pos.col--
	case pb.Direction_DIRECTION_RIGHT:
		pos.col++
	}

	if pos.col < 0 ||
		pos.col >= s.game.width ||
		pos.row < 0 ||
		pos.row >= s.game.height {
		s.mu.Unlock()

		return &pb.MoveResponse{
			Ok: false,
		}, nil
	}

	if s.game.tiles[pos.row][pos.col] == tileWall {
		s.mu.Unlock()
		return &pb.MoveResponse{Ok: false}, nil
	}

	for id, p := range s.game.positions {
		if id != req.PlayerId && p == pos {
			s.mu.Unlock()
			return &pb.MoveResponse{Ok: false}, nil
		}
	}

	s.game.positions[req.PlayerId] = pos

	clients := make([]*client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}

	s.mu.Unlock()

	event := &pb.Event{
		Type:      pb.EventType_EVENT_TYPE_MOVE,
		PlayerId:  req.PlayerId,
		Direction: req.Direction,
		X:         int32(pos.col),
		Y:         int32(pos.row),
	}

	broadcast(clients, event)

	return &pb.MoveResponse{Ok: true}, nil
}

func (s *server) SendMessage(
	ctx context.Context,
	req *pb.SendMessageRequest,
) (*pb.SendMessageResponse, error) {
	if _, joined := s.getClient(req.PlayerId); !joined {
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	event := &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_CHAT,
		PlayerId: req.PlayerId,
		Message:  req.Message,
	}

	broadcast(s.snapshotClients(), event)

	fmt.Printf("[%s] %s\r\n", req.PlayerId, req.Message)

	return &pb.SendMessageResponse{Ok: true}, nil
}

func (s *server) Leave(
	ctx context.Context,
	req *pb.LeaveRequest,
) (*pb.LeaveResponse, error) {
	s.mu.Lock()

	if _, joined := s.clients[req.PlayerId]; !joined {
		s.mu.Unlock()
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	delete(s.clients, req.PlayerId)

	clients := make([]*client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}

	delete(s.game.positions, req.PlayerId)

	s.mu.Unlock()

	message := fmt.Sprintf("%s left", req.PlayerId)

	event := &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_PLAYER_LEFT,
		PlayerId: req.PlayerId,
		Message:  message,
	}

	broadcast(clients, event)

	fmt.Printf("%s left\r\n", req.PlayerId)

	return &pb.LeaveResponse{
		Ok: true,
	}, nil
}

func (s *server) Subscribe(
	req *pb.SubscribeRequest,
	stream pb.ChatService_SubscribeServer,
) error {
	client, joined := s.getClient(req.PlayerId)
	if !joined {
		return fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	for {
		select {
		case message := <-client.messages:
			if err := stream.Send(message); err != nil {
				return err
			}

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()

	chatServer := &server{
		clients: make(map[string]*client),
		game:    newGameState(10, 10),
	}

	pb.RegisterChatServiceServer(
		grpcServer,
		chatServer,
	)

	fmt.Println("Server listening on :50051")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
