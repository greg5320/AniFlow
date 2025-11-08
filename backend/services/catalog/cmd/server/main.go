package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	// "time"
	"strings"

	kodik "github.com/greg5320/AniFlow/backend/services/catalog/internal/kodik" 
	pb "github.com/greg5320/AniFlow/backend/services/catalog/gen" 
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedCatalogServer
	client *kodik.Client
}

func isKodikID(s string) bool {
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "movie-") || strings.HasPrefix(s, "serial-")
}
func canonicalKey(m kodik.Material) string {
	if m.KinopoiskID != "" {
		return "kp:" + m.KinopoiskID
	}
	if m.ShikimoriID != "" {
		return "sh:" + m.ShikimoriID
	}
	return fmt.Sprintf("ttl:%s|%d", strings.ToLower(strings.TrimSpace(m.Title)), m.Year)
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

	if req.Query != "" && isKodikID(req.Query) {
		mat, err := s.client.FetchByID(ctx, req.Query, true)
		if err != nil {
			return nil, err
		}
		pbItem := &pb.Anime{
			KodikId:      mat.ID,
			Title:        mat.Title,
			Description:  mat.Description,
			PosterUrl:    mat.PosterURL,
			EpisodesCount: int32(mat.EpisodesCount),
			UpdatedAt:    timestamppb.Now(),
		}
		if mat.Translation != nil {
			pbItem.Translations = append(pbItem.Translations, &pb.Translation{
				Id:    int32(mat.Translation.ID),
				Title: mat.Translation.Title,
				Type:  mat.Translation.Type,
			})
		}
		return &pb.SearchResponse{
			Items: []*pb.Anime{pbItem},
			Total: 1,
		}, nil
	}

	lr, err := s.client.Search(ctx, req.Query, pageSize, true) 
	if err != nil {
		return nil, err
	}

	type agg struct {
		Representative kodik.Material
		Translations   map[int]kodik.Translation 
		Count          int
	}
	mmap := make(map[string]*agg)

	for _, m := range lr.Results {
		key := canonicalKey(m)
		a, ok := mmap[key]
		if !ok {
			a = &agg{
				Representative: m,
				Translations:   make(map[int]kodik.Translation),
				Count:          0,
			}
			mmap[key] = a
		}
		a.Count++
		if m.Translation != nil {
			a.Translations[m.Translation.ID] = *m.Translation
		}
	}


	resp := &pb.SearchResponse{}
	for _, a := range mmap {
		rep := a.Representative
		item := &pb.Anime{
			KodikId:      rep.ID, 
			Title:        rep.Title,
			Description:  rep.Description,
			PosterUrl:    rep.PosterURL,
			EpisodesCount: int32(rep.EpisodesCount),
			UpdatedAt:    timestamppb.Now(),
		}
		for _, tr := range a.Translations {
			item.Translations = append(item.Translations, &pb.Translation{
				Id:    int32(tr.ID),
				Title: tr.Title,
				Type:  tr.Type,
			})
		}
		resp.Items = append(resp.Items, item)
	}
	resp.Total = int32(len(resp.Items))
	return resp, nil
}
func (s *server) GetAnime(ctx context.Context, req *pb.GetAnimeRequest) (*pb.Anime, error) {
	if req == nil || req.KodikId == "" {
		return nil, fmt.Errorf("kodik_id is required")
	}

	mat, err := s.client.FetchByID(ctx, req.KodikId, true)
	if err != nil {
		return nil, err
	}

	var translationsMap = make(map[int]kodik.Translation)
	if mat.KinopoiskID != "" {
		lr, err := s.client.SearchByKinopoiskID(ctx, mat.KinopoiskID, 100, true)
		if err == nil {
			for _, m := range lr.Results {
				if m.Translation != nil {
					translationsMap[m.Translation.ID] = *m.Translation
				}
				if mat.PosterURL == "" && m.PosterURL != "" {
					mat.PosterURL = m.PosterURL
				}
				if mat.Description == "" && m.Description != "" {
					mat.Description = m.Description
				}
			}
		}
	} else {
		if mat.Translation != nil {
			translationsMap[mat.Translation.ID] = *mat.Translation
		}
	}

	anime := &pb.Anime{
		KodikId:      mat.ID,
		Title:        mat.Title,
		Description:  mat.Description,
		PosterUrl:    mat.PosterURL,
		EpisodesCount: int32(mat.EpisodesCount),
		UpdatedAt:    timestamppb.Now(),
	}

	for _, tr := range translationsMap {
		anime.Translations = append(anime.Translations, &pb.Translation{
			Id:    int32(tr.ID),
			Title: tr.Title,
			Type:  tr.Type,
		})
	}

	return anime, nil
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
