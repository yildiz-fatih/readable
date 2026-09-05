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
	s3Client              *s3.Client
	s3PresignClient       *s3.PresignClient
	s3BucketName          string
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

	seaweedfsInternalURL := os.Getenv("SEAWEEDFS_INTERNAL_URL")
	if seaweedfsInternalURL == "" {
		logger.Error("SEAWEEDFS_INTERNAL_URL is not set")
		os.Exit(1)
	}

	seaweedfsPublicURL := os.Getenv("SEAWEEDFS_PUBLIC_URL")
	if seaweedfsPublicURL == "" {
		logger.Error("SEAWEEDFS_PUBLIC_URL is not set")
		os.Exit(1)
	}

	s3BucketName := os.Getenv("S3_BUCKET_NAME")
	if s3BucketName == "" {
		logger.Error("S3_BUCKET_NAME is not set")
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

	s3Client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(seaweedfsInternalURL)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	s3PresignClient := s3.NewPresignClient(s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(seaweedfsPublicURL)
		o.UsePathStyle = true
	}))

	app := &application{
		logger:                logger,
		httpClient:            httpClient,
		safeHttpClient:        safeHttpClient,
		readabilityServiceURL: readabilityServiceURL,
		gotenbergURL:          gotenbergURL,
		epubServiceURL:        epubServiceURL,
		s3Client:              s3Client,
		s3PresignClient:       s3PresignClient,
		s3BucketName:          s3BucketName,
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
