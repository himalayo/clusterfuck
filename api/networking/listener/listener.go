package api

import (
	"context"
	"log"
	"time"

	pb "github.com/himalayo/clusterfuck/api/networking/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SendRequest struct {
	packet *pb.Packet
	result chan bool
}

type Listener struct {
	packet     chan SendRequest
	HandlerIds []int
}

func NewListener() *Listener {
	return &Listener{
		packet: make(chan SendRequest),
	}
}

func (n *Listener) sendData(client pb.IncomingListenerClient, packet *pb.Packet) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	succ, err := client.ReceivePacket(ctx, packet)
	if err != nil {
		log.Printf("could not send packet: %v", err)
		return false
	}
	return succ.Successful
}

func (n *Listener) Send(sso string, data []byte) bool {
	result_channel := make(chan bool)
	n.packet <- SendRequest{packet: &pb.Packet{ClientId: sso, Packet: data}, result: result_channel}
	result := <-result_channel
	close(result_channel)
	return result
}

func (n *Listener) Listen(addr string) error {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	n.ListenWithConnection(conn)
	return nil
}

func (n *Listener) ListenWithConnection(conn *grpc.ClientConn) {
	defer conn.Close()
	c := pb.NewIncomingListenerClient(conn)
	for req := range n.packet {
		go func() {
			req.result <- n.sendData(c, req.packet)
		}()
	}
}
