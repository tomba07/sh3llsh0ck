package shellfire

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/tomba07/bombshell/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedChatServiceServer
}

func (s *server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	fmt.Printf("%s joined\n", req.Name)

	return &pb.JoinResponse{
		PlayerId: req.Name,
	}, nil
}

func (s *server) SendMessage(
	ctx context.Context,
	req *pb.SendMessageRequest,
) (*pb.SendMessageResponse, error) {
	fmt.Printf("[%s] %s\n", req.PlayerId, req.Message)

	return &pb.SendMessageResponse{
		Ok: true,
	}, nil
}

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterChatServiceServer(
		grpcServer,
		&server{},
	)

	fmt.Println("Server listening on :50051")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
