package api

import (
	"context"
	"log"

	pb "github.com/himalayo/clusterfuck/api/items/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ItemsClient struct {
	client pb.ItemsClient
}

func NewClient() *ItemsClient {
	return &ItemsClient{}
}

func (n *ItemsClient) GetItemById(id int) (*pb.Item, error) {
	return n.client.GetItem(context.Background(), &pb.ItemRequest{
		Request: &pb.ItemRequest_ItemId{
			ItemId: int32(id),
		},
	})
}

func (n *ItemsClient) GetItemByName(name string) (*pb.Item, error) {
	return n.client.GetItem(context.Background(), &pb.ItemRequest{
		Request: &pb.ItemRequest_ItemName{
			ItemName: name,
		},
	})
}

func (n *ItemsClient) GetItems() (*pb.ItemList, error) {
	return n.client.GetItems(context.Background(), &pb.Empty{})
}

func (n *ItemsClient) GetItemByIds(ids []int) (*pb.ItemList, error) {
	ids_i32 := make([]int32, len(ids))
	for i, id := range ids {
		ids_i32[i] = int32(id)
	}

	return n.client.GetItemList(context.Background(), &pb.ItemListRequest{
		ItemIds: ids_i32,
	})
}

func (n *ItemsClient) Listen(addr string) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("could not connect to networking module: %v", err)
	}
	n.client = pb.NewItemsClient(conn)
}
