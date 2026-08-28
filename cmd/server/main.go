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
}

type client struct {
	name     string
	messages chan *pb.Event
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

	s.mu.Unlock()

	message := fmt.Sprintf("%s joined", req.Name)

	event := &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_PLAYER_JOINED,
		PlayerId: req.Name,
		Message:  message,
	}

	for _, client := range clients {
		client.messages <- event
	}

	fmt.Printf("%s joined\n", req.Name)

	return &pb.JoinResponse{
		PlayerId: req.Name,
	}, nil
}

func (s *server) SendMessage(
	ctx context.Context,
	req *pb.SendMessageRequest,
) (*pb.SendMessageResponse, error) {
	s.mu.Lock()

	if _, joined := s.clients[req.PlayerId]; !joined {
		s.mu.Unlock()
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	clients := make([]*client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}

	s.mu.Unlock()

	message := &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_CHAT,
		PlayerId: req.PlayerId,
		Message:  req.Message,
	}

	for _, client := range clients {
		client.messages <- message
	}

	fmt.Printf("[%s] %s\n", req.PlayerId, req.Message)

	return &pb.SendMessageResponse{
		Ok: true,
	}, nil
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

	s.mu.Unlock()

	message := fmt.Sprintf("%s left", req.PlayerId)

	event := &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_PLAYER_LEFT,
		PlayerId: req.PlayerId,
		Message:  message,
	}

	for _, client := range clients {
		client.messages <- event
	}

	fmt.Printf("%s left\n", req.PlayerId)

	return &pb.LeaveResponse{
		Ok: true,
	}, nil
}

func (s *server) Subscribe(
	req *pb.SubscribeRequest,
	stream pb.ChatService_SubscribeServer,
) error {
	s.mu.Lock()
	client, joined := s.clients[req.PlayerId]
	//don't use defer here due to the endless loop
	s.mu.Unlock()

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
