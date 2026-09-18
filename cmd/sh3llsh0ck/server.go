package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/tomba07/bombshell/proto"
	"google.golang.org/grpc"
)

type gameServer struct {
	pb.UnimplementedChatServiceServer

	mu          sync.Mutex
	clients     map[string]*gameClient
	game        gameState
	scores      map[string]int
	trapCancels map[string]context.CancelFunc
}

type gameClient struct {
	name     string
	messages chan *pb.Event
}

var trapCounter atomic.Int64

func nextTrapID() string {
	return fmt.Sprintf("trap-%d", trapCounter.Add(1))
}

func startServer(port int) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	s := &gameServer{
		clients:     make(map[string]*gameClient),
		game:        newGameState(30, 16),
		scores:      make(map[string]int),
		trapCancels: make(map[string]context.CancelFunc),
	}
	pb.RegisterChatServiceServer(grpcServer, s)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func (s *gameServer) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	s.mu.Lock()

	if _, exists := s.clients[req.Name]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("name %q already taken", req.Name)
	}

	s.clients[req.Name] = &gameClient{
		name:     req.Name,
		messages: make(chan *pb.Event, 10),
	}

	clients := make([]*gameClient, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}

	best, ok := s.game.bestSpawn()
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("no available spawn positions")
	}

	for id, pos := range s.game.positions {
		s.clients[req.Name].messages <- &pb.Event{
			Type:     pb.EventType_EVENT_TYPE_PLAYER_JOINED,
			PlayerId: id,
			Col:      int32(pos.col),
			Row:      int32(pos.row),
		}
	}

	s.game.positions[req.Name] = best
	s.mu.Unlock()

	pos := s.game.positions[req.Name]
	broadcast(clients, &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_PLAYER_JOINED,
		PlayerId: req.Name,
		Message:  fmt.Sprintf("%s joined", req.Name),
		Col:      int32(pos.col),
		Row:      int32(pos.row),
	})

	fmt.Printf("%s joined\r\n", req.Name)

	var walls []*pb.TilePos
	for row := range s.game.height {
		for col := range s.game.width {
			if s.game.tiles[row][col] == tileWall {
				walls = append(walls, &pb.TilePos{Col: int32(col), Row: int32(row)})
			}
		}
	}

	return &pb.JoinResponse{
		PlayerId: req.Name,
		Width:    int32(s.game.width),
		Height:   int32(s.game.height),
		Walls:    walls,
	}, nil
}

func (s *gameServer) Subscribe(req *pb.SubscribeRequest, stream pb.ChatService_SubscribeServer) error {
	s.mu.Lock()
	c, joined := s.clients[req.PlayerId]
	s.mu.Unlock()
	if !joined {
		return fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	for {
		select {
		case message := <-c.messages:
			if err := stream.Send(message); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (s *gameServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
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

	if pos.col < 0 || pos.col >= s.game.width || pos.row < 0 || pos.row >= s.game.height {
		s.mu.Unlock()
		return &pb.MoveResponse{Ok: false}, nil
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

	clients := make([]*gameClient, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	broadcast(clients, &pb.Event{
		Type:      pb.EventType_EVENT_TYPE_MOVE,
		PlayerId:  req.PlayerId,
		Direction: req.Direction,
		Col:       int32(pos.col),
		Row:       int32(pos.row),
	})

	return &pb.MoveResponse{Ok: true}, nil
}

func (s *gameServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	s.mu.Lock()
	_, joined := s.clients[req.PlayerId]
	clients := make([]*gameClient, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	if !joined {
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	broadcast(clients, &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_CHAT,
		PlayerId: req.PlayerId,
		Message:  req.Message,
	})

	fmt.Printf("[%s] %s\r\n", req.PlayerId, req.Message)
	return &pb.SendMessageResponse{Ok: true}, nil
}

func (s *gameServer) Leave(ctx context.Context, req *pb.LeaveRequest) (*pb.LeaveResponse, error) {
	s.mu.Lock()

	if _, joined := s.clients[req.PlayerId]; !joined {
		s.mu.Unlock()
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	delete(s.clients, req.PlayerId)
	delete(s.game.positions, req.PlayerId)

	clients := make([]*gameClient, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	broadcast(clients, &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_PLAYER_LEFT,
		PlayerId: req.PlayerId,
		Message:  fmt.Sprintf("%s left", req.PlayerId),
	})

	fmt.Printf("%s left\r\n", req.PlayerId)
	return &pb.LeaveResponse{Ok: true}, nil
}

func (s *gameServer) PlaceTrap(ctx context.Context, req *pb.PlaceTrapRequest) (*pb.PlaceTrapResponse, error) {
	s.mu.Lock()

	if _, joined := s.game.positions[req.PlayerId]; !joined {
		s.mu.Unlock()
		return nil, fmt.Errorf("player %q is not joined", req.PlayerId)
	}

	trapID := nextTrapID()
	t, ok := s.game.placeTrap(req.PlayerId, trapID)
	if !ok {
		s.mu.Unlock()
		return &pb.PlaceTrapResponse{Ok: false}, nil
	}

	clients := make([]*gameClient, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	broadcast(clients, &pb.Event{
		Type:     pb.EventType_EVENT_TYPE_TRAP_PLACED,
		PlayerId: req.PlayerId,
		TrapId:   trapID,
		Col:      int32(t.col),
		Row:      int32(t.row),
	})

	s.scheduleTrap(t)
	return &pb.PlaceTrapResponse{Ok: true, TrapId: trapID}, nil
}

func (s *gameServer) scheduleTrap(t trap) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.trapCancels[t.id] = cancel
	s.mu.Unlock()

	go func() {
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return
		}

		s.mu.Lock()
		delete(s.trapCancels, t.id)
		clients := make([]*gameClient, 0, len(s.clients))
		for _, c := range s.clients {
			clients = append(clients, c)
		}
		// Process the full chain and collect all blasts, hits, and scores.
		queue := []trap{t}
		var allBlasts []*pb.TrapBlast
		allHits := make(map[string]position) // playerID -> respawn position
		scoreUpdates := make(map[string]int)

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]

			if cancel, ok := s.trapCancels[cur.id]; ok {
				cancel()
				delete(s.trapCancels, cur.id)
			}

			hit, respawns, chained := s.game.detonate(cur)
			queue = append(queue, chained...)

			allBlasts = append(allBlasts, &pb.TrapBlast{
				TrapId:  cur.id,
				OwnerId: cur.ownerID,
				Col:     int32(cur.col),
				Row:     int32(cur.row),
			})

			kills := 0
			for _, id := range hit {
				if id != cur.ownerID {
					kills++
				}
				allHits[id] = respawns[id]
			}
			if kills > 0 {
				scoreUpdates[cur.ownerID] += kills
			}
		}

		for ownerID, kills := range scoreUpdates {
			s.scores[ownerID] += kills
		}

		broadcast(clients, &pb.Event{
			Type:        pb.EventType_EVENT_TYPE_TRAP_TRIGGERED,
			PlayerId:    t.ownerID,
			BlastRadius: int32(blastRadius),
			Blasts:      allBlasts,
		})

		for ownerID := range scoreUpdates {
			broadcast(clients, &pb.Event{
				Type:     pb.EventType_EVENT_TYPE_SCORE_UPDATE,
				PlayerId: ownerID,
				Score:    int32(s.scores[ownerID]),
			})
		}

		for id, newPos := range allHits {
			broadcast(clients, &pb.Event{
				Type:     pb.EventType_EVENT_TYPE_CHAT,
				PlayerId: id,
				Message:  fmt.Sprintf("%s was hit", id),
			})
			broadcast(clients, &pb.Event{
				Type:     pb.EventType_EVENT_TYPE_PLAYER_JOINED,
				PlayerId: id,
				Col:      int32(newPos.col),
				Row:      int32(newPos.row),
			})
		}
		s.mu.Unlock()
	}()
}

func broadcast(clients []*gameClient, event *pb.Event) {
	for _, c := range clients {
		c.messages <- event
	}
}
