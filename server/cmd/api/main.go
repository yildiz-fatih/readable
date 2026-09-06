package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type application struct {
	logger      *slog.Logger
	riverClient *river.Client[pgx.Tx]
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load("../.env")

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	postgresURL := os.Getenv("POSTGRES_URL")
	if postgresURL == "" {
		logger.Error("POSTGRES_URL is not set")
		os.Exit(1)
	}

	dbPool, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	err = dbPool.Ping(context.Background())
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("connected to database")

	riverClient, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{}) // insert-only river client
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	app := &application{
		logger:      logger,
		riverClient: riverClient,
	}

	server := &http.Server{
		Addr:     ":" + port,
		Handler:  app.newRouter(),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "address", server.Addr)
	err = server.ListenAndServe() // err is always non-nil
	logger.Error(err.Error())
	os.Exit(1)
}
