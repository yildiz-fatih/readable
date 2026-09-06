package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/yildiz-fatih/readable/server/internal/jobs"
)

type createReadableRequest struct {
	URL    string `json:"url"`
	Format string `json:"format"` // "html", "pdf", "epub"
}

type createReadableResponse struct {
	JobID string `json:"job_id"`
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

	if req.Format != "html" && req.Format != "pdf" && req.Format != "epub" {
		app.clientError(w, http.StatusBadRequest, "unsupported format")
		return
	}

	jobInsertResult, err := app.riverClient.Insert(r.Context(), jobs.ReadableArgs{
		URL:    req.URL,
		Format: req.Format,
	}, nil)
	if err != nil {
		app.serverError(w, err)
		return
	}

	jobId := strconv.FormatInt(jobInsertResult.Job.ID, 10)
	res := createReadableResponse{JobID: jobId}

	err = writeJSON(w, http.StatusAccepted, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) getReadableHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not yet implemented", http.StatusNotImplemented)
}
