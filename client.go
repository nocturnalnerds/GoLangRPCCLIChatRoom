package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	pb "GOCLIAPP/chatpb"

	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("0.tcp.ngrok.io:14417", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("❌ could not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("👤 Enter your name: ")
	username, _ := reader.ReadString('\n')
	username = username[:len(username)-1]

	stream, err := client.Join(context.Background(), &pb.User{Name: username})
	if err != nil {
		log.Fatalf("❌ error joining: %v", err)
	}

	fmt.Printf("Welcome %s.\n", username);

	// Broadcast join message to all users
	joinMsg := &pb.Message{
		User:      username,
		Text:      fmt.Sprintf("%s has joined the chat.", username),
		Timestamp: time.Now().Unix(),
	}
	_, err = client.SendMessage(context.Background(), joinMsg)
	if err != nil {
		log.Printf("❌ failed to send join message: %v", err)
	}

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				log.Fatalf("❌ error receiving message: %v", err)
			}
			if msg.User != username {
				fmt.Printf("[%s] %s\n", msg.User, msg.Text)
			}
		}
	}()

	for {
		text, _ := reader.ReadString('\n')
		text = text[:len(text)-1]

		if text == "" {
			continue
		}
		
		if len(text) > 0 && text[0] == '/' {
			if text == "/exit" {
				fmt.Println("👋 Exiting...")
				leaveMsg := &pb.Message{
					User:      username,
					Text:      fmt.Sprintf("%s has left the chat.", username),
					Timestamp: time.Now().Unix(),
				}
				_, err := client.SendMessage(context.Background(), leaveMsg)
				if err != nil {
					log.Printf("❌ failed to send leave message: %v", err)
				}
				os.Exit(0)
			}
			fmt.Println("⚠️  Special command detected:", text)
			// Handle other special commands here
			continue
		}

		msg := &pb.Message{
			User:      username,
			Text:      text,
			Timestamp: time.Now().Unix(),
		}
		_, err := client.SendMessage(context.Background(), msg)
		if err != nil {
			log.Fatalf("❌ failed to send message: %v", err)
		}
	}
}
