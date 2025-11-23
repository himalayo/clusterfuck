package api

import (
	"context"
	"log"
	"time"

	pb "github.com/himalayo/clusterfuck/api/permission/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PermissionsClient struct {
	client pb.PermissionsClient
}

func NewClient() *PermissionsClient {
	return &PermissionsClient{}
}

func (n *PermissionsClient) GetRank(id int) *pb.Rank {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := n.client.GetRank(ctx, &pb.RankId{Id: int32(id)})
	if err != nil {
		log.Printf("error getting rank: %v", err)
		return nil
	}
	return out
}

func (n *PermissionsClient) GetRankByName(name string) *pb.Rank {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := n.client.GetRankByName(ctx, &pb.RankName{Name: name})
	if err != nil {
		return nil
	}
	return out
}

func (n *PermissionsClient) GetAllRanks() *pb.RankList {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := n.client.GetAllRanks(ctx, &pb.Empty{})
	if err != nil {
		return nil
	}
	return out
}

func (n *PermissionsClient) CheckRankExists(rankId int) *pb.RankExists {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := n.client.CheckRankExists(ctx, &pb.RankId{Id: int32(rankId)})
	if err != nil {
		return nil
	}
	return out
}

func (n *PermissionsClient) GetPermission(rankId int, key string) *pb.Permission {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := n.client.GetPermission(ctx, &pb.RankPermission{RankId: int32(rankId), PermissionKey: key})
	if err != nil {
		return nil
	}
	return out
}

func (n *PermissionsClient) GetRankLevel(rankId int) *pb.Level {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := n.client.GetRankLevel(ctx, &pb.RankId{Id: int32(rankId)})
	if err != nil {
		return nil
	}
	return out
}

func (n *PermissionsClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	n.client = pb.NewPermissionsClient(conn)
	log.Printf("PermissionClient.Listen: Listening for data with address: %s", addr)
}
