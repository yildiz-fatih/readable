package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/yildiz-fatih/readable/server/internal/jobs"
	"github.com/yildiz-fatih/readable/server/internal/models"
)

type createReadableRequest struct {
	URL    string `json:"url"`
	Format string `json:"format"` // "html", "pdf", "epub"
}

type createReadableResponse struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

func (app *application) createReadableHandler(w http.ResponseWriter, r *http.Request) {
	var req createReadableRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}
	defer r.Body.Close()

	if req.URL == "" {
		app.clientError(w, http.StatusBadRequest, "'url' is required")
		return
	}

	if req.Format != string(models.HTML) && req.Format != string(models.PDF) && req.Format != string(models.EPUB) {
		app.clientError(w, http.StatusBadRequest, "unsupported format")
		return
	}

	tx, err := app.db.Begin(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer tx.Rollback(r.Context())

	// --- transaction start ---

	// enqueue the job
	jobInsertResult, err := app.riverClient.InsertTx(r.Context(), tx, jobs.ReadableArgs{
		URL:    req.URL,
		Format: req.Format,
	}, &river.InsertOpts{
		MaxAttempts: 3,
	})
	if err != nil {
		app.serverError(w, err)
		return
	}
	jobId := int(jobInsertResult.Job.ID)

	// insert into readables
	query := "INSERT INTO readables (id, format) VALUES ($1, $2) RETURNING id, status, format, created"
	var readable models.Readable
	err = tx.QueryRow(r.Context(), query, jobId, req.Format).Scan(&readable.ID, &readable.Status, &readable.Format, &readable.Created)
	if err != nil {
		app.serverError(w, err)
		return
	}

	// --- transaction end ---

	err = tx.Commit(r.Context())
	if err != nil {
		app.serverError(w, err)
		return
	}

	res := createReadableResponse{
		ID:     jobId,
		Status: string(readable.Status),
	}
	err = writeJSON(w, http.StatusAccepted, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) getReadableHandler(w http.ResponseWriter, r *http.Request) {
	type getReadableResponse struct {
		Status string `json:"status"`
		URL    string `json:"url,omitempty"`
	}

	id := r.PathValue("id")
	if _, err := strconv.Atoi(id); err != nil {
		app.clientError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	// check job status
	query := `SELECT id, status, format, created
	FROM readables
	WHERE id = $1`

	var readable models.Readable
	err := app.db.QueryRow(r.Context(), query, id).Scan(&readable.ID, &readable.Status, &readable.Format, &readable.Created)
	if err != nil {
		// invalid job ID check
		if errors.Is(err, pgx.ErrNoRows) {
			app.clientError(w, http.StatusNotFound, "Readable is not found")
			return
		}

		app.serverError(w, err)
		return
	}

	switch readable.Status {
	case models.Succeeded:
		presignedReq, err := app.s3PresignClient.PresignGetObject(r.Context(), &s3.GetObjectInput{
			Bucket:                     aws.String(app.s3BucketName),
			Key:                        aws.String(id),
			ResponseContentDisposition: aws.String(fmt.Sprintf(`inline; filename="%s.%s"`, id, readable.Format)),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = 1 * time.Hour
		})
		if err != nil {
			app.serverError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, nil, getReadableResponse{Status: string(models.Succeeded), URL: presignedReq.URL})
		return
	case models.Pending:
		writeJSON(w, http.StatusOK, nil, getReadableResponse{Status: string(models.Pending)})
		return
	case models.Failed:
		writeJSON(w, http.StatusOK, nil, getReadableResponse{Status: string(models.Failed)})
		return
	}
}
