package api

import (
	"context"
	"log"

	pb "github.com/himalayo/clusterfuck/api/catalog/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CatalogClient struct {
	client pb.CatalogClient
}

func (n *CatalogClient) GetCatalogPageByPageId(pageId int) (*pb.CatalogPage, error) {
	return n.client.GetCatalogPage(context.Background(), &pb.CatalogPageRequest{
		Request: &pb.CatalogPageRequest_PageId{
			PageId: int32(pageId),
		},
	})
}

func (n *CatalogClient) GetCatalogPageByName(caption string) (*pb.CatalogPage, error) {
	return n.client.GetCatalogPage(context.Background(), &pb.CatalogPageRequest{
		Request: &pb.CatalogPageRequest_CaptionSafe{
			CaptionSafe: caption,
		},
	})
}

func (n *CatalogClient) GetCatalogPageByLayout(layoutName string) (*pb.CatalogPage, error) {
	return n.client.GetCatalogPage(context.Background(), &pb.CatalogPageRequest{
		Request: &pb.CatalogPageRequest_LayoutName{
			LayoutName: layoutName,
		},
	})
}

func (n *CatalogClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	n.client = pb.NewCatalogClient(conn)
}
