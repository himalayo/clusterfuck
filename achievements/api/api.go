package api


import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "github.com/himalayo/clusterfuck/achievements/proto"
)

type AchievementsClient struct {
	client pb.AchievementsClient
}

func NewClient() *AchievementsClient {
	return &AchievementsClient{}
}

func (a *AchievementsClient) GetAchievement(id int) (*pb.Achievement, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	achievement, err := a.client.GetAchievement(ctx, &pb.AchievementId{Id: int32(id)})
	if err != nil {
		return nil, err
	}
	return achievement, nil
}


func (a *AchievementsClient) GetAchievementByName(name string) (*pb.Achievement, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	achievement, err := a.client.GetAchievementByName(ctx, &pb.AchievementName{Name: name})
	if err != nil {
		return nil, err
	}
	return achievement, nil
}


func (a *AchievementsClient) InventoryAchievementsComposer() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	packet, err := a.client.InventoryAchievementsComposer(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}
	return packet.GetData(), nil
}

func (a *AchievementsClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	a.client = pb.NewAchievementsClient(conn)
}
