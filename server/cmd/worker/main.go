package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/doyensec/safeurl"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/yildiz-fatih/readable/server/internal/jobs"
)

type ReadableWorker struct {
	// An embedded WorkerDefaults sets up default methods to fulfill the rest of
	// the Worker interface:
	river.WorkerDefaults[jobs.ReadableArgs]
	// other stuff
	httpClient            *http.Client
	safeHttpClient        *safeurl.WrappedClient
	readabilityServiceURL string
	s3Client              *s3.Client
	s3PresignClient       *s3.PresignClient
	s3BucketName          string
	epubServiceURL        string
	gotenbergURL          string
	logger                *slog.Logger
}

func (w *ReadableWorker) Work(ctx context.Context, job *river.Job[jobs.ReadableArgs]) error {
	fetchReq, err := http.NewRequestWithContext(ctx, http.MethodGet, job.Args.URL, nil)
	if err != nil {
		w.logger.Error(err.Error())
		return err
	}
	fetchReq.Header.Set("User-Agent", "readable/1.0")

	fetchRes, err := w.safeHttpClient.Do(fetchReq)
	if err != nil {
		w.logger.Error(err.Error())
		return err
	}
	defer fetchRes.Body.Close()

	const maxHtmlSize int = 10 * 1024 * 1024 // 10 MB
	html, err := io.ReadAll(io.LimitReader(fetchRes.Body, int64(maxHtmlSize)+1))
	if err != nil {
		w.logger.Error(err.Error())
		return err
	}
	if len(html) > maxHtmlSize {
		err = fmt.Errorf("got HTML of %d bytes, want a maximum of: %d bytes", len(html), maxHtmlSize)
		w.logger.Error(err.Error())
		return err
	}

	readabilityReqBody, err := json.Marshal(map[string]string{"url": job.Args.URL, "html": string(html)})
	if err != nil {
		w.logger.Error(err.Error())
		return err
	}
	readabilityReq, err := http.NewRequestWithContext(ctx, http.MethodPost, w.readabilityServiceURL+"/extract", bytes.NewReader(readabilityReqBody))
	if err != nil {
		w.logger.Error(err.Error())
		return err
	}
	readabilityReq.Header.Set("Content-Type", "application/json")

	readabilityRes, err := w.httpClient.Do(readabilityReq)
	if err != nil {
		w.logger.Error(err.Error())
		return err
	}
	defer readabilityRes.Body.Close()

	if readabilityRes.StatusCode != http.StatusOK {
		err = fmt.Errorf("readability service returned status: %d", readabilityRes.StatusCode)
		w.logger.Error(err.Error())
		return err
	}

	var link string

	switch job.Args.Format {
	case "html":
		htmlBytes, err := io.ReadAll(readabilityRes.Body)
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		link, err = w.uploadAndPresign(ctx, htmlBytes, "text/html", "html")
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
	case "pdf":
		var buf bytes.Buffer
		multipartWriter := multipart.NewWriter(&buf)
		form, err := multipartWriter.CreateFormFile("files", "index.html")
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		_, err = io.Copy(form, readabilityRes.Body)
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		err = multipartWriter.Close()
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		gotenbergReq, err := http.NewRequestWithContext(ctx, http.MethodPost, w.gotenbergURL+"/forms/chromium/convert/html", &buf)
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		gotenbergReq.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		gotenbergRes, err := w.httpClient.Do(gotenbergReq)
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		defer gotenbergRes.Body.Close()
		if gotenbergRes.StatusCode != http.StatusOK {
			err = fmt.Errorf("gotenberg returned status: %d", gotenbergRes.StatusCode)
			w.logger.Error(err.Error())
			return err
		}
		pdfBytes, err := io.ReadAll(gotenbergRes.Body)
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		link, err = w.uploadAndPresign(ctx, pdfBytes, "application/pdf", "pdf")
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
	case "epub":
		epubServiceReq, err := http.NewRequestWithContext(ctx, http.MethodPost, w.epubServiceURL+"/html-to-epub", readabilityRes.Body)
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		epubServiceReq.Header.Set("Content-Type", "text/html")
		epubServiceRes, err := w.httpClient.Do(epubServiceReq)
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		defer epubServiceRes.Body.Close()
		if epubServiceRes.StatusCode != http.StatusOK {
			err = fmt.Errorf("epub service returned status: %d", epubServiceRes.StatusCode)
			w.logger.Error(err.Error())
			return err
		}
		epubBytes, err := io.ReadAll(epubServiceRes.Body)
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
		link, err = w.uploadAndPresign(ctx, epubBytes, "application/epub+zip", "epub")
		if err != nil {
			w.logger.Error(err.Error())
			return err
		}
	}

	/*
	* TODO: write "link" to postgres, so that the GET endpoint can find it later.
	 */
	w.logger.Info("link is ready", "url", link)

	// success
	return nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load("../.env")

	postgresURL := os.Getenv("POSTGRES_URL")
	if postgresURL == "" {
		logger.Error("POSTGRES_URL is not set")
		os.Exit(1)
	}

	readabilityServiceURL := os.Getenv("READABILITY_SERVICE_URL")
	if readabilityServiceURL == "" {
		logger.Error("READABILITY_SERVICE_URL is not set")
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

	s3InternalURL := os.Getenv("S3_INTERNAL_URL")
	if s3InternalURL == "" {
		logger.Error("S3_INTERNAL_URL is not set")
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

	epubServiceURL := os.Getenv("EPUB_SERVICE_URL")
	if epubServiceURL == "" {
		logger.Error("EPUB_SERVICE_URL is not set")
		os.Exit(1)
	}

	gotenbergURL := os.Getenv("GOTENBERG_URL")
	if gotenbergURL == "" {
		logger.Error("GOTENBERG_URL is not set")
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	safeHttpClient := safeurl.Client(safeurl.GetConfigBuilder().SetTimeout(30 * time.Second).Build()) // for SSRF

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
		o.BaseEndpoint = aws.String(s3InternalURL)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	s3PresignClient := s3.NewPresignClient(s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(s3PublicURL)
		o.UsePathStyle = true
	}))

	worker := &ReadableWorker{
		httpClient:            httpClient,
		safeHttpClient:        safeHttpClient,
		readabilityServiceURL: readabilityServiceURL,
		s3Client:              s3Client,
		s3PresignClient:       s3PresignClient,
		s3BucketName:          s3BucketName,
		epubServiceURL:        epubServiceURL,
		gotenbergURL:          gotenbergURL,
		logger:                logger,
	}

	workers := river.NewWorkers()
	// AddWorker panics if the worker is already registered or invalid:
	river.AddWorker(workers, worker)

	dbPool, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	riverClient, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 100},
		},
		Workers: workers,
	})
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := riverClient.Start(signalCtx); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	<-riverClient.Stopped()
}

func (w *ReadableWorker) uploadAndPresign(ctx context.Context, body []byte, contentType, ext string) (string, error) {
	key := uuid.New().String()

	_, err := w.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(w.s3BucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	presignedReq, err := w.s3PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(w.s3BucketName),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(fmt.Sprintf(`inline; filename="%s.%s"`, key, ext)),
	}, s3.WithPresignExpires(24*time.Hour))
	if err != nil {
		return "", err
	}

	return presignedReq.URL, nil
}
