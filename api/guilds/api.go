package api

import (
	"context"
	"log"

	pb "github.com/himalayo/clusterfuck/api/guilds/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GuildsClient struct {
	client pb.GuildsClient
}

func (n *GuildsClient) GetBases() ([]*pb.GuildPart, error) {
	res, err := n.client.GetBases(context.Background(), &pb.Empty{})
	return res.GetParts(), err
}

func (n *GuildsClient) GetSymbols() ([]*pb.GuildPart, error) {
	res, err := n.client.GetSymbols(context.Background(), &pb.Empty{})
	return res.GetParts(), err
}

func (n *GuildsClient) GetBaseColors() ([]*pb.GuildPart, error) {
	res, err := n.client.GetBaseColors(context.Background(), &pb.Empty{})
	return res.GetParts(), err
}

func (n *GuildsClient) GetSymbolColors() ([]*pb.GuildPart, error) {
	res, err := n.client.GetSymbolColors(context.Background(), &pb.Empty{})
	return res.GetParts(), err
}

func (n *GuildsClient) GetBackgroundColors() ([]*pb.GuildPart, error) {
	res, err := n.client.GetBackgroundColors(context.Background(), &pb.Empty{})
	return res.GetParts(), err
}

func (n *GuildsClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	n.client = pb.NewGuildsClient(conn)
}
