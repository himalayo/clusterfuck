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

	pb "github.com/himalayo/clusterfuck/api/catalog/proto"
	"google.golang.org/grpc"
)

var (
	grpc_port = flag.Int("grpc_port", 50061, "The gRPC server port")
)

type server struct {
	pb.UnimplementedCatalogServer
	data *Database
}

func (p *CatalogPage) ToProto() *pb.CatalogPage {
	if p == nil {
		return nil
	}

	return &pb.CatalogPage{
		Id:           int32(p.Id),
		ParentId:     int32(p.ParentId),
		Rank:         int32(p.Rank),
		Caption:      p.Caption,
		PageName:     p.PageName,
		IconColor:    int32(p.IconColor),
		IconImage:    int32(p.IconImage),
		Visible:      p.Visible,
		Enabled:      p.Enabled,
		ClubOnly:     p.ClubOnly,
		Layout:       p.Layout,
		HeaderImage:  p.HeaderImage,
		TeaserImage:  p.TeaserImage,
		SpecialImage: p.SpecialImage,
		TextOne:      p.TextOne,
		TextTwo:      p.TextTwo,
		TextDetails:  p.TextDetails,
		TextTeaser:   p.TextTeaser,
	}
}

func CatalogPageResultToProto(page *CatalogPage, err error) (*pb.CatalogPage, error) {
	return page.ToProto(), err
}

func (s *server) GetCatalogPage(ctx context.Context, in *pb.CatalogPageRequest) (*pb.CatalogPage, error) {
	if req, ok := in.Request.(*pb.CatalogPageRequest_PageId); ok {
		return CatalogPageResultToProto(data.GetCatalogPageByPageId(ctx, int(req.PageId)))
	}

	if req, ok := in.Request.(*pb.CatalogPageRequest_CaptionSafe); ok {
		return CatalogPageResultToProto(data.GetCatalogPageByName(ctx, req.CaptionSafe))
	}

	if req, ok := in.Request.(*pb.CatalogPageRequest_LayoutName); ok {
		return CatalogPageResultToProto(data.GetCatalogPageByLayout(ctx, req.LayoutName))
	}

	return nil, nil
}

func StartServer(data *Database) {
	_, portString, _ := strings.Cut(os.Getenv("GUILDS_HOST"), ":")
	port, err := strconv.Atoi(portString)
	if err != nil {
		port = *grpc_port
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Could not start gRPC socket: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterCatalogServer(s, &server{data: data})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC error: %v", err)
	}
}
