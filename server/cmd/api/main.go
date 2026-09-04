package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/doyensec/safeurl"
	"github.com/joho/godotenv"
)

type application struct {
	logger                *slog.Logger
	httpClient            *http.Client
	safeHttpClient        *safeurl.WrappedClient
	readabilityServiceURL string
	gotenbergURL          string
	epubServiceURL        string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	httpClient := &http.Client{Timeout: 30 * time.Second}
	safeHttpClient := safeurl.Client(safeurl.GetConfigBuilder().SetTimeout(30 * time.Second).Build()) // for SSRF

	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	readabilityServiceURL := os.Getenv("READABILITY_SERVICE_URL")
	if readabilityServiceURL == "" {
		logger.Error("READABILITY_SERVICE_URL is not set")
		os.Exit(1)
	}

	gotenbergURL := os.Getenv("GOTENBERG_URL")
	if gotenbergURL == "" {
		logger.Error("GOTENBERG_URL is not set")
		os.Exit(1)
	}

	epubServiceURL := os.Getenv("EPUB_SERVICE_URL")
	if epubServiceURL == "" {
		logger.Error("EPUB_SERVICE_URL is not set")
		os.Exit(1)
	}

	app := &application{
		logger:                logger,
		httpClient:            httpClient,
		safeHttpClient:        safeHttpClient,
		readabilityServiceURL: readabilityServiceURL,
		gotenbergURL:          gotenbergURL,
		epubServiceURL:        epubServiceURL,
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
