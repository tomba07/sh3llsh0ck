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
	x int
	y int
}

type gameState struct {
	width     int
	height    int
	tiles     [][]tile
	positions map[string]position
}

func newGameState(width, height int) gameState {
	tiles := make([][]tile, height)

	for y := 0; y < height; y++ {
		tiles[y] = make([]tile, width)
	}

	for x := 0; x < width; x++ {
		tiles[0][x] = tileWall
		tiles[height-1][x] = tileWall
	}

	for y := 0; y < height; y++ {
		tiles[y][0] = tileWall
		tiles[y][width-1] = tileWall
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

	s.game.positions[req.Name] = position{
		x: 1,
		y: 1,
	}

	s.mu.Unlock()

	message := fmt.Sprintf("%s joined", req.Name)
	pos := s.game.positions[req.Name]
	event := &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_PLAYER_JOINED,
		PlayerId: req.Name,
		Message:  message,
		X:        int32(pos.x),
		Y:        int32(pos.y),
	}

	broadcast(clients, event)

	fmt.Printf("%s joined\r\n", req.Name)

	return &pb.JoinResponse{
		PlayerId: req.Name,
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
		pos.y--
	case pb.Direction_DIRECTION_DOWN:
		pos.y++
	case pb.Direction_DIRECTION_LEFT:
		pos.x--
	case pb.Direction_DIRECTION_RIGHT:
		pos.x++
	}

	if pos.x < 0 ||
		pos.x >= s.game.width ||
		pos.y < 0 ||
		pos.y >= s.game.height {
		s.mu.Unlock()

		return &pb.MoveResponse{
			Ok: false,
		}, nil
	}

	if s.game.tiles[pos.y][pos.x] == tileWall {
		s.mu.Unlock()
		return &pb.MoveResponse{Ok: false}, nil
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
		X:         int32(pos.x),
		Y:         int32(pos.y),
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
