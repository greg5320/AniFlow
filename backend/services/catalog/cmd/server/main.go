package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	// "time"
	"sort"
	"strings"

	structpb "google.golang.org/protobuf/types/known/structpb"
	kodik "github.com/greg5320/AniFlow/backend/services/catalog/internal/kodik" 
	pb "github.com/greg5320/AniFlow/backend/services/catalog/gen" 
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	pb.UnimplementedCatalogServer
	client *kodik.Client
}

// func isKodikID(s string) bool {
// 	if s == "" {
// 		return false
// 	}
// 	return strings.HasPrefix(s, "movie-") || strings.HasPrefix(s, "serial-")
// }
func canonicalKey(m kodik.Material) string {
	if m.KinopoiskID != "" {
		return "kp:" + m.KinopoiskID
	}
	// if m.ShikimoriID != "" {
	// 	return "sh:" + m.ShikimoriID
	// }
	return fmt.Sprintf("ttl:%s|%d", strings.ToLower(strings.TrimSpace(m.Title)), m.Year)
}
func (s *server) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	lr, err := s.client.Search(ctx, req.Query, pageSize, true)
	if err != nil {
		return nil, err
	}
	type agg struct {
		Rep          kodik.Material
		Translations map[int]kodik.Translation
	}
	mmap := make(map[string]*agg)

	for _, m := range lr.Results {
		key := canonicalKey(m)
		a, ok := mmap[key]
		if !ok {
			a = &agg{Rep: m, Translations: make(map[int]kodik.Translation)}
			mmap[key] = a
		}
		if m.Translation != nil {
			a.Translations[m.Translation.ID] = *m.Translation
		}
		if a.Rep.PosterURL == "" && m.PosterURL != "" {
			a.Rep.PosterURL = m.PosterURL
		}
		if a.Rep.Description == "" && m.Description != "" {
			a.Rep.Description = m.Description
		}
		if len(a.Rep.Genres) == 0 && len(m.Genres) > 0 {
			a.Rep.Genres = m.Genres
		}
		if m.KinopoiskRating > a.Rep.KinopoiskRating {
			a.Rep.KinopoiskRating = m.KinopoiskRating
		}
	}

	keys := make([]string, 0, len(mmap))
	for k := range mmap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return mmap[keys[i]].Rep.Title < mmap[keys[j]].Rep.Title
	})

	resp := &pb.SearchResponse{}
	for _, k := range keys {
		a := mmap[k]
		rep := a.Rep
		item := &pb.Anime{
			KodikId:       rep.ID,
			Title:         rep.Title,
			Description:   rep.Description,
			PosterUrl:     rep.PosterURL,
			EpisodesCount: int32(rep.EpisodesCount),
			Year:          int32(rep.Year),
			Genres: rep.Genres,
			UpdatedAt: timestamppb.Now(),
		}
		if rep.KinopoiskRating > 0 {
			item.KinopoiskRating = rep.KinopoiskRating
		} else {
			item.KinopoiskRating = 0
		}
		if rep.AnimePosterURL != "" {
			item.AnimePosterUrl = rep.AnimePosterURL
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
		return nil, fmt.Errorf("kodik_id required")
	}

	mat, err := s.client.FetchByID(ctx, req.KodikId, true)
	if err != nil {
		return nil, err
	}

	transMap := make(map[int]kodik.Translation)
	if mat.Translation != nil {
		transMap[mat.Translation.ID] = *mat.Translation
	}

	if mat.KinopoiskID != "" {
		lr, err := s.client.SearchByKinopoiskID(ctx, mat.KinopoiskID, 200, true)
		if err == nil {
			for _, mm := range lr.Results {
				if canonicalKey(mm) != canonicalKey(*mat) {
					continue
				}
				if mm.Translation != nil {
					transMap[mm.Translation.ID] = *mm.Translation
				}
				if mat.PosterURL == "" && mm.PosterURL != "" {
					mat.PosterURL = mm.PosterURL
				}
				if mat.AnimePosterURL == "" && mm.AnimePosterURL != "" {
					mat.AnimePosterURL = mm.AnimePosterURL
				}
				if len(mat.Genres) == 0 && len(mm.Genres) > 0 {
					mat.Genres = mm.Genres
				}
				if mm.KinopoiskRating > mat.KinopoiskRating {
					mat.KinopoiskRating = mm.KinopoiskRating
				}
			}
		}
	} else {
		lr, err := s.client.Search(ctx, mat.Title, 50, true)
		if err == nil {
			for _, mm := range lr.Results {
				if canonicalKey(mm) != canonicalKey(*mat) {
					continue
				}
				if mm.Translation != nil {
					transMap[mm.Translation.ID] = *mm.Translation
				}
			}
		}
	}

	var fullData *structpb.Struct
	if mat.Raw != nil {
		sv, convErr := structpb.NewStruct(mat.Raw)
		if convErr != nil {
			fmt.Printf("[warn] failed to convert raw to structpb: %v\n", convErr)
		} else {
			fullData = sv
		}
	}

	out := &pb.Anime{
		KodikId:       req.KodikId,
		Title:         mat.Title,
		Description:   mat.Description,
		PosterUrl:     mat.PosterURL,
		EpisodesCount: int32(mat.EpisodesCount),
		Year:          int32(mat.Year),
		Genres:        mat.Genres,
		UpdatedAt:     timestamppb.Now(),
		KinopoiskRating: mat.KinopoiskRating,
		AnimePosterUrl:  mat.AnimePosterURL,
		FullData:        fullData,
	}

	ids := make([]int, 0, len(transMap))
	for id := range transMap {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		tr := transMap[id]
		out.Translations = append(out.Translations, &pb.Translation{
			Id:    int32(tr.ID),
			Title: tr.Title,
			Type:  tr.Type,
		})
	}

	return out, nil
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
