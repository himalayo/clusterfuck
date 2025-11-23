package api

import (
	"context"
	"log"
	"time"

	pb "github.com/himalayo/clusterfuck/api/subscription/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SubscriptionClient struct {
	client          pb.SubscriptionClient
	userId          chan int
	Subscriptions   chan []*pb.SubscriptionInstance
	checkClub       chan *pb.SubscriptionRequest
	setActive       chan *pb.ActivationRequest
	addDuration     chan *pb.DurationRequest
	HasSubscription chan bool
	Active          chan *pb.SubscriptionInstance
	DurationAdded   chan *pb.SubscriptionInstance
}

func NewClient() *SubscriptionClient {
	return &SubscriptionClient{
		userId:          make(chan int),
		Subscriptions:   make(chan []*pb.SubscriptionInstance),
		checkClub:       make(chan *pb.SubscriptionRequest),
		setActive:       make(chan *pb.ActivationRequest),
		addDuration:     make(chan *pb.DurationRequest),
		HasSubscription: make(chan bool),
		Active:          make(chan *pb.SubscriptionInstance),
		DurationAdded:   make(chan *pb.SubscriptionInstance),
	}
}

func (n *SubscriptionClient) GetSubscriptions(userId int) []*pb.SubscriptionInstance {
	n.userId <- userId
	subs := <-n.Subscriptions
	return subs
}

func (n *SubscriptionClient) UserHasSubscription(userId int, subscriptionType string) bool {
	n.checkClub <- &pb.SubscriptionRequest{UserId: int32(userId), SubscriptionType: subscriptionType}
	result := <-n.HasSubscription
	return result
}

func (n *SubscriptionClient) SetActive(subscriptionId int, active bool) *pb.SubscriptionInstance {
	n.setActive <- &pb.ActivationRequest{SubscriptionId: int32(subscriptionId), Active: active}
	result := <-n.Active
	return result
}

func (n *SubscriptionClient) AddDuration(subscriptionId int, amount int) *pb.SubscriptionInstance {
	n.addDuration <- &pb.DurationRequest{SubscriptionId: int32(subscriptionId), Duration: int32(amount)}
	result := <-n.DurationAdded
	return result
}

func (n *SubscriptionClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	defer conn.Close()
	n.client = pb.NewSubscriptionClient(conn)
	log.Printf("SubscriptionClient.Listen: Listening for data with address: %s", addr)
	for {
		select {
		case id := <-n.userId:
			go func(n *SubscriptionClient, id int) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				subs, err := n.client.GetSubscripitonsForUser(ctx, &pb.User{Id: int32(id)})
				if err != nil {
					n.Subscriptions <- nil
					return
				}
				n.Subscriptions <- subs.Subscriptions
			}(n, id)
		case req := <-n.checkClub:
			go func(n *SubscriptionClient, req *pb.SubscriptionRequest) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				subs, err := n.client.UserHasSubscription(ctx, req)
				if err != nil {
					n.HasSubscription <- false
					return
				}
				n.HasSubscription <- subs.HasSubscription
			}(n, req)
		case req := <-n.setActive:
			go func(n *SubscriptionClient, req *pb.ActivationRequest) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				sub, err := n.client.SetActive(ctx, req)
				if err != nil {
					n.Active <- nil
					return
				}
				n.Active <- sub
			}(n, req)
		case req := <-n.addDuration:
			go func(n *SubscriptionClient, req *pb.DurationRequest) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				sub, err := n.client.AddDuration(ctx, req)
				if err != nil {
					n.DurationAdded <- nil
					return
				}
				n.DurationAdded <- sub
			}(n, req)

		}
	}
}
