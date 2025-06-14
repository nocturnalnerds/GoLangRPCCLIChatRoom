package main

import (
  "context"
  "crypto/ed25519"
  "crypto/rand"
  "fmt"
  "log"
  "net"
  "sync"

  pb "GOCLIAPP/chatpb"
  "google.golang.org/grpc"
)

type clientInfo struct {
  channel string
  msgChan chan *pb.Message
}

type chatServer struct {
  pb.UnimplementedChatServiceServer
  mu        sync.Mutex
  clients   map[string]*clientInfo
  rooms     map[string]bool
  users     map[string]ed25519.PublicKey
  nonces    map[string][]byte
}

func newServer() *chatServer {
  return &chatServer{
    clients: make(map[string]*clientInfo),
    rooms:   map[string]bool{"main": true},
    users:   make(map[string]ed25519.PublicKey),
    nonces:  make(map[string][]byte),
  }
}

func (s *chatServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
  s.mu.Lock()
  defer s.mu.Unlock()
  if _, exists := s.users[req.Name]; exists {
    return &pb.RegisterResponse{Success: false, Message: "already registered"}, nil
  }
  s.users[req.Name] = ed25519.PublicKey(req.Pubkey)
  return &pb.RegisterResponse{Success: true, Message: "registered"}, nil
}

func (s *chatServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
  s.mu.Lock()
  defer s.mu.Unlock()
  if _, exists := s.users[req.Name]; !exists {
    return &pb.LoginResponse{Success: false, Message: "not registered"}, nil
  }
  nonce := make([]byte, 32)
  rand.Read(nonce)
  s.nonces[req.Name] = nonce
  return &pb.LoginResponse{Nonce: nonce, Success: true, Message: "nonce issued"}, nil
}

func (s *chatServer) Verify(ctx context.Context, req *pb.VerifyRequest) (*pb.VerifyResponse, error) {
  s.mu.Lock()
  defer s.mu.Unlock()
  pub, ok := s.users[req.Name]
  if !ok {
    return &pb.VerifyResponse{Success: false, Message: "not registered"}, nil
  }
  nonce, ok := s.nonces[req.Name]
  if !ok {
    return &pb.VerifyResponse{Success: false, Message: "login not initiated"}, nil
  }
  if !ed25519.Verify(pub, nonce, req.Signed) {
    return &pb.VerifyResponse{Success: false, Message: "invalid signature"}, nil
  }
  delete(s.nonces, req.Name)
  return &pb.VerifyResponse{Success: true, Message: "login successful"}, nil
}

func (s *chatServer) CheckUsername(ctx context.Context, user *pb.User) (*pb.CheckResponse, error) {
  s.mu.Lock()
  defer s.mu.Unlock()
  _, taken := s.users[user.Name]
  return &pb.CheckResponse{IsTaken: taken}, nil
}

func (s *chatServer) CreateRoom(ctx context.Context, req *pb.RoomRequest) (*pb.RoomResponse, error) {
  s.mu.Lock()
  defer s.mu.Unlock()
  if s.rooms[req.Name] {
    return &pb.RoomResponse{Created: false, AlreadyExists: true}, nil
  }
  s.rooms[req.Name] = true
  return &pb.RoomResponse{Created: true}, nil
}

func (s *chatServer) SwitchChannel(ctx context.Context, u *pb.User) (*pb.Empty, error) {
  s.mu.Lock()
  defer s.mu.Unlock()
  ci, ok := s.clients[u.Name]
  if !ok {
    return nil, fmt.Errorf("user not active")
  }
  if !s.rooms[u.Channel] {
    return nil, fmt.Errorf("room not exists")
  }
  ci.channel = u.Channel
  return &pb.Empty{}, nil
}

func (s *chatServer) SendMessage(ctx context.Context, m *pb.Message) (*pb.Empty, error) {
  s.mu.Lock()
  defer s.mu.Unlock()
  if m.Receiver != "" {
    if ch, ok := s.clients[m.Receiver]; ok {
      ch.msgChan <- m
    }
    return &pb.Empty{}, nil
  }
  for _, ch := range s.clients {
    if ch.channel == m.Channel {
      ch.msgChan <- m
    }
  }
  return &pb.Empty{}, nil
}

func (s *chatServer) Join(u *pb.User, stream pb.ChatService_JoinServer) error {
  s.mu.Lock()
  if _, ok := s.clients[u.Name]; ok {
    s.mu.Unlock()
    return fmt.Errorf("already connected")
  }
  ci := &clientInfo{channel: u.Channel, msgChan: make(chan *pb.Message, 100)}
  s.clients[u.Name] = ci
  s.mu.Unlock()

  defer func() {
    s.mu.Lock()
    delete(s.clients, u.Name)
    s.mu.Unlock()
  }()

  for {
    select {
    case <-stream.Context().Done():
      return nil
    case msg := <-ci.msgChan:
      stream.Send(msg)
    }
  }
}

func main() {
  lis, err := net.Listen("tcp", ":50051")
  if err != nil {
    log.Fatalf("Failed to listen: %v", err)
  }
  grpcServer := grpc.NewServer()
  pb.RegisterChatServiceServer(grpcServer, newServer())
  log.Println("Server started on port 50051")
  grpcServer.Serve(lis)
}
