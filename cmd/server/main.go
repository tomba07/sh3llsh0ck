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
	name string
}

func (s *server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[req.Name]; exists {
		return nil, fmt.Errorf("name %q already taken", req.Name)
	}

	s.clients[req.Name] = &client{
		name: req.Name,
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
	defer s.mu.Unlock()

	if _, joined := s.clients[req.PlayerId]; !joined {
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
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
	defer s.mu.Unlock()

	if _, joined := s.clients[req.PlayerId]; !joined {
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	delete(s.clients, req.PlayerId)

	fmt.Printf("%s left\n", req.PlayerId)

	return &pb.LeaveResponse{
		Ok: true,
	}, nil
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
