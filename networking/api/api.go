package api

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "github.com/himalayo/clusterfuck/networking/proto"
)

type Packet struct {
	sso string
	body []byte
}

type NetworkingClient struct {
	send chan Packet
}

func NewClient() (*NetworkingClient) {
	return &NetworkingClient {
		send: make(chan Packet),
	}
}

func sendPacket(client pb.NetworkingClient, msg *pb.Packet) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	succ, err := client.SendPacket(ctx, msg)
	if err != nil {
		log.Printf("could not send packet: %v", err)
		return false
	}
	return succ.Successful
}

func sendPackets(client pb.NetworkingClient, msgs *pb.Packets) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	succ, err := client.SendPackets(ctx, msgs)
	if err != nil {
		log.Printf("could not send packet: %v", err)
		return false
	}
	return succ.Successful
}

func (n *NetworkingClient) Send(sso string, data []byte) {
	n.send <- Packet{sso: sso, body: data}
}

func (n *NetworkingClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	defer conn.Close()
	c := pb.NewNetworkingClient(conn)
	for {
		select {
		case packet := <- n.send:
			sendPacket(c, &pb.Packet{ClientId: packet.sso, Packet: packet.body})
		}
	}
}
