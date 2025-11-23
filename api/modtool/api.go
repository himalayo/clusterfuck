package api

import (
	"context"
	"log"
	"time"

	pb "github.com/himalayo/clusterfuck/api/modtool/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ModtoolClient struct {
	client         pb.ModtoolClient
	compose        chan struct{}
	message        chan []byte
	getTopic       chan int
	resultingTopic chan *pb.CfhTopic
}

func NewClient() *ModtoolClient {
	return &ModtoolClient{compose: make(chan struct{}), message: make(chan []byte)}
}

func (n *ModtoolClient) CfhTopicsMessageComposer() []byte {
	n.compose <- struct{}{}
	result := <-n.message
	return result
}

func (n *ModtoolClient) GetCfhTopic(id int) *pb.CfhTopic {
	n.getTopic <- id
	result := <-n.resultingTopic
	return result
}

func (n *ModtoolClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	defer conn.Close()
	n.client = pb.NewModtoolClient(conn)
	for {
		select {
		case topicId := <-n.getTopic:
			go func(id int) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				topic, err := n.client.GetCfhTopic(ctx, &pb.TopicId{Id: int32(id)})
				if err != nil {
					log.Printf("GetCfhTopic: %v", err)
					n.resultingTopic <- nil
					return
				}
				n.resultingTopic <- topic
			}(topicId)
		case _ = <-n.compose:
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				packet, err := n.client.CfhTopicsMessageComposer(ctx, &pb.Empty{})
				if err != nil {
					log.Printf("CfhTopicsMessageComposer: %v", err)
					n.message <- nil
					return
				}
				n.message <- packet.Data
			}()
		}
	}
}
