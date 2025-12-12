package api

import (
	"context"
	"log"
	"time"

	pb "github.com/himalayo/clusterfuck/api/player/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PlayerClient struct {
	client       pb.PlayerClient
	loginRequest chan string
	Login        chan string
}

func NewClient() *PlayerClient {
	return &PlayerClient{Login: make(chan string), loginRequest: make(chan string)}
}

func (n *PlayerClient) sendLoginRequest(ticket *pb.Ticket) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ok, err := n.client.LoginPlayer(ctx, ticket)
	if err != nil {
		log.Fatalf("could not login player: %v", err)
	}
	return ok.Success
}

func (n *PlayerClient) sendUserDataRequest(ticket *pb.Ticket) *pb.UserData {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ok, err := n.client.GetUserData(ctx, ticket)
	if err != nil {
		return nil
	}
	return ok
}

func (n *PlayerClient) LoginRequest(sso string) {
	n.loginRequest <- sso
}

func (n *PlayerClient) GetUserData(sso string) *pb.UserData {
	return n.sendUserDataRequest(&pb.Ticket{Sso: sso})
}

func (n *PlayerClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	defer conn.Close()
	n.client = pb.NewPlayerClient(conn)
	log.Printf("PlayerClient.Listen: Listening for data with address: %s", addr)
	for {
		select {
		case sso := <-n.loginRequest:
			response := n.sendLoginRequest(&pb.Ticket{Sso: sso})
			if response {
				n.Login <- sso
			}
		}
	}
}
