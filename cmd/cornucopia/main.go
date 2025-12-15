package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	_ "github.com/go-sql-driver/mysql"
	google_grpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/traP-jp/plutus/api/protobuf"
	"github.com/traP-jp/plutus/system/cornucopia/internal/handler/grpc"
	"github.com/traP-jp/plutus/system/cornucopia/internal/infrastructure/repository"
	"github.com/traP-jp/plutus/system/cornucopia/internal/usecase"
)

func main() {
	// Config
	dbUser := os.Getenv("MYSQL_USER")
	dbPass := os.Getenv("MYSQL_PASSWORD")
	dbName := os.Getenv("MYSQL_DATABASE")
	dbHost := "localhost" // or use env

	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?parseTime=true", dbUser, dbPass, dbHost, dbName)
	
	// Database
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()
	
	if err := db.Ping(); err != nil {
		log.Printf("warning: db ping failed: %v", err)
	}

	// Repositories
	repo := repository.NewMariaDBRepository(db)

	// UseCases
	transferUC := usecase.NewTransferUseCase(repo, repo, repo)
	accountUC := usecase.NewAccountUseCase(repo, repo)

	// Handlers
	h := grpc.NewCornucopiaHandler(transferUC, accountUC)

	// gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := google_grpc.NewServer()
	pb.RegisterCornucopiaServiceServer(s, h)
	reflection.Register(s)

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
