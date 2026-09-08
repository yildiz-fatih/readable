package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/yildiz-fatih/readable/server/internal/repository"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 20 * time.Second
	idleTimeout       = 120 * time.Second
)

type application struct {
	logger             *slog.Logger
	riverClient        *river.Client[pgx.Tx]
	db                 *pgxpool.Pool
	s3PresignClient    *s3.PresignClient
	s3BucketName       string
	readableRepository *repository.ReadableRepository
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

	s3PublicURL := os.Getenv("S3_PUBLIC_URL")
	if s3PublicURL == "" {
		logger.Error("S3_PUBLIC_URL is not set")
		os.Exit(1)
	}

	s3BucketName := os.Getenv("S3_BUCKET")
	if s3BucketName == "" {
		logger.Error("S3_BUCKET is not set")
		os.Exit(1)
	}

	awsAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	if awsAccessKeyID == "" {
		logger.Error("AWS_ACCESS_KEY_ID is not set")
		os.Exit(1)
	}

	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if awsSecretAccessKey == "" {
		logger.Error("AWS_SECRET_ACCESS_KEY is not set")
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

	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(awsAccessKeyID, awsSecretAccessKey, ""),
		),
	)
	if err != nil {
		logger.Error("failed to load AWS SDK config", "error", err)
		os.Exit(1)
	}

	s3PresignClient := s3.NewPresignClient(s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(s3PublicURL)
		o.UsePathStyle = true
	}))

	app := &application{
		logger:             logger,
		riverClient:        riverClient,
		db:                 dbPool,
		s3PresignClient:    s3PresignClient,
		s3BucketName:       s3BucketName,
		readableRepository: repository.NewReadableRepository(dbPool),
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           app.newRouter(),
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	logger.Info("starting server", "address", server.Addr)
	err = server.ListenAndServe() // err is always non-nil
	logger.Error(err.Error())
	os.Exit(1)
}
