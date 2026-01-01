package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	pb "github.com/himalayo/clusterfuck/api/items/proto"
	"google.golang.org/grpc"
)

var (
	grpc_port = flag.Int("grpc_port", 50062, "The gRPC server port")
)

type server struct {
	pb.UnimplementedItemsServer
	data *Database
}

func (item Item) ToProto() *pb.Item {
	vending_ids_strings := strings.Split(item.VendingIds, ";")
	vending_ids := make([]int32, 0, len(vending_ids_strings))
	for _, vending_id_string := range vending_ids_strings {
		vending_id, err := strconv.ParseInt(vending_id_string, 10, 32)
		if err == nil {
			vending_ids = append(vending_ids, int32(vending_id))
		}
	}
	multiheights := []float64{}
	if strings.Contains(item.MultiHeight, ";") {
		multiheights_strings := strings.Split(item.MultiHeight, ";")
		multiheights = make([]float64, 0, len(multiheights_strings))
		for _, multiheight_string := range multiheights_strings {
			multiheight, err := strconv.ParseFloat(multiheight_string, 64)
			if err == nil {
				multiheights = append(multiheights, multiheight)
			}
		}
	}

	return &pb.Item{
		Id:                  int32(item.Id),
		SpriteId:            int32(item.SpriteId),
		Name:                item.ItemName,
		FullName:            item.PublicName,
		FurnitureType:       item.Type,
		Width:               int32(item.Width),
		Length:              int32(item.Length),
		Height:              item.Height,
		AllowStack:          item.AllowStack,
		AllowWalk:           item.AllowWalk == 1,
		AllowSit:            item.AllowSit == 1,
		AllowLay:            item.AllowLay == 1,
		AllowRecycle:        item.AllowRecycle,
		AllowTrade:          item.AllowTrade,
		AllowMarketplace:    item.AllowMarketplace,
		AllowGift:           item.AllowGift,
		AllowInventoryStack: item.AllowInventoryStack,
		StateCount:          int32(item.StateCount),
		EffectM:             int32(item.EffectM),
		EffectF:             int32(item.EffectF),
		VendingItems:        vending_ids,
		MultiHeights:        multiheights,
		CustomParams:        item.CustomParams,
		ClothingOnWalk:      item.ClothingOnWalk,
		InteractionType:     item.InteractionType,
		Serialized:          item.Serialize(),
	}
}

func ItemsResultToProto(items []Item, err error) (*pb.ItemList, error) {
	if err != nil {
		return nil, err
	}
	xs := make([]*pb.Item, len(items))
	for i, item := range items {
		xs[i] = item.ToProto()
	}
	return &pb.ItemList{Items: xs}, nil
}

func ItemResultToProto(item *Item, err error) (*pb.Item, error) {
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return item.ToProto(), nil
}

func (s *server) GetItem(ctx context.Context, in *pb.ItemRequest) (*pb.Item, error) {
	if req, ok := in.Request.(*pb.ItemRequest_ItemId); ok {
		return ItemResultToProto(s.data.GetItemById(ctx, int(req.ItemId)))
	}

	if req, ok := in.Request.(*pb.ItemRequest_ItemName); ok {
		return ItemResultToProto(s.data.GetItemByName(ctx, req.ItemName))
	}

	return nil, nil
}

func (s *server) GetItems(ctx context.Context, _ *pb.Empty) (*pb.ItemList, error) {
	return ItemsResultToProto(s.data.GetItems(ctx))
}

func (s *server) GetItemList(ctx context.Context, in *pb.ItemListRequest) (*pb.ItemList, error) {
	return ItemsResultToProto(s.data.GetItemByIds(ctx, in.ItemIds))
}

func StartServer(data *Database) {
	_, portString, _ := strings.Cut(os.Getenv("ITEMS_HOST"), ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *grpc_port
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Could not start gRPC socket: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterItemsServer(s, &server{data: data})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}
}
