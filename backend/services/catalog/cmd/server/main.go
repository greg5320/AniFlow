// AniFlow/backend/services/catalog/cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	// "time"

	kodik "github.com/greg5320/AniFlow/backend/services/catalog/internal/kodik" 
	pb "github.com/greg5320/AniFlow/backend/services/catalog/gen" 
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedCatalogServer
	client *kodik.Client
}

func (s *server) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	page := int(req.Page)
	if page < 1 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	types := "anime"

	var next string
	var lastRes *kodik.ListResponse
	for p := 1; p <= page; p++ {
		lr, err := s.client.FetchPage(ctx, pageSize, next, types, false, false)
		if err != nil {
			return nil, err
		}
		lastRes = lr
		if lr.NextPage == nil || *lr.NextPage == "" {
			next = ""
			break
		}
		next = *lr.NextPage
	}
	if lastRes == nil {
		return &pb.SearchResponse{}, nil
	}

	resp := &pb.SearchResponse{}
	for _, m := range lastRes.Results {
		anime := &pb.Anime{
			KodikId:      m.ID,
			Title:        m.Title,
			Description:  m.Description,
			PosterUrl:    m.PosterURL,
			EpisodesCount: int32(m.EpisodesCount),
			UpdatedAt:    timestamppb.Now(),
		}
		resp.Items = append(resp.Items, anime)
	}
	resp.Total = int32(lastRes.Total)
	return resp, nil
}

func (s *server) GetAnime(ctx context.Context, req *pb.GetAnimeRequest) (*pb.Anime, error) {
	 err := s.client.FetchPage(ctx, 1, "", "", false, true)
	if err != nil {
		return nil, err
	}

	return &pb.Anime{
		KodikId: req.KodikId,
		Title:   "TODO implement GetAnime properly (FetchByID)",
	}, nil
}

func main() {
	token := os.Getenv("KODIK_API_TOKEN")
	if token == "" {
		log.Fatal("KODIK_API_TOKEN is not set")
	}
	portStr := os.Getenv("CATALOG_PORT")
	if portStr == "" {
		log.Fatal("CATALOG_PORT is not set")
	}
	port, _ := strconv.Atoi(portStr)

	client := kodik.NewClient(token)
	srv := &server{client: client}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterCatalogServer(grpcServer, srv)

	log.Printf("catalog gRPC server listening on :%d", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve error: %v", err)
	}
}
