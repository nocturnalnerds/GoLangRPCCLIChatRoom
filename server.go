package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	pb "GOCLIAPP/chatpb"

	"google.golang.org/grpc"
)

type chatServer struct {
	pb.UnimplementedChatServiceServer
	mu      sync.Mutex
	clients map[string]chan *pb.Message
}

func newServer() *chatServer {
	return &chatServer{
		clients: make(map[string]chan *pb.Message),
	}
}

func (s *chatServer) SendMessage(ctx context.Context, msg *pb.Message) (*pb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.clients {
		ch <- msg
	}
	return &pb.Empty{}, nil
}

func (s *chatServer) Join(user *pb.User, stream pb.ChatService_JoinServer) error {
	msgChan := make(chan *pb.Message, 100)
	s.mu.Lock()
	s.clients[user.Name] = msgChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, user.Name)
		s.mu.Unlock()
	}()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case msg := <-msgChan:
			if err := stream.Send(msg); err != nil {
				log.Printf("Error sending message: %v", err)
			}
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterChatServiceServer(grpcServer, newServer())

	fmt.Println("🚀 Chat server listening on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
