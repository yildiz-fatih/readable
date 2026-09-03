package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type application struct {
	logger *slog.Logger
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := &application{
		logger: logger,
	}

	server := &http.Server{
		Addr:     ":" + port,
		Handler:  app.newRouter(),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "address", server.Addr)
	err := server.ListenAndServe() // err is always non-nil
	logger.Error(err.Error())
	os.Exit(1)
}
