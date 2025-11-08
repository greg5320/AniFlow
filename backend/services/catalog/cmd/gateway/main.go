package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	pb "github.com/greg5320/AniFlow/backend/services/catalog/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/connectivity"
)

func main() {
	grpcAddr := os.Getenv("CATALOG_GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = "localhost:50051"
	}

	cc, err := grpc.NewClient(grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to create gRPC client: %v", err)
	}
	defer cc.Close()

	cc.Connect()

	ctxWait, cancelWait := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelWait()
	for {
		state := cc.GetState()
		if state == connectivity.Ready {
			log.Printf("gRPC connection to %s is READY", grpcAddr)
			break
		}
		if !cc.WaitForStateChange(ctxWait, state) {
			log.Printf("timed out waiting for connection state change; current state=%s", cc.GetState())
			break
		}
	}

	client := pb.NewCatalogClient(cc)

	r := gin.Default()

	// POST /v1/search
	r.POST("/v1/search", func(c *gin.Context) {
		var req struct {
			Query    string `json:"query"`
			Page     int32  `json:"page"`
			PageSize int32  `json:"page_size"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Page == 0 {
			req.Page = 1
		}
		if req.PageSize == 0 {
			req.PageSize = 20
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		grpcReq := &pb.SearchRequest{
			Query:    req.Query,
			Page:     req.Page,
			PageSize: req.PageSize,
		}
		grpcResp, err := client.Search(ctx, grpcReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, grpcResp)
	})

	// GET /v1/anime/:kodik_id
	r.GET("/v1/anime/:kodik_id", func(c *gin.Context) {
		kid := c.Param("kodik_id")
		if kid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "kodik_id required"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		grpcResp, err := client.GetAnime(ctx, &pb.GetAnimeRequest{KodikId: kid})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		b, err := json.Marshal(grpcResp)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json", b)
	})

	httpPort := os.Getenv("GATEWAY_PORT")
	log.Printf("gateway listening on :%s, proxying to %s", httpPort, grpcAddr)
	if err := r.Run(":" + httpPort); err != nil {
		log.Fatalf("gateway failed: %v", err)
	}
}
