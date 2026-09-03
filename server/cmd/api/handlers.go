package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (app *application) createReadableHandler(w http.ResponseWriter, r *http.Request) {
	type createReadableRequest struct {
		URL string `json:"url"`
	}
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

	fetchReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, req.URL, nil)
	if err != nil {
		app.serverError(w, err)
		return
	}
	fetchReq.Header.Set("User-Agent", "readable/1.0")

	fetchRes, err := app.safeHttpClient.Do(fetchReq)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer fetchRes.Body.Close()

	const maxHtmlSize int = 10 * 1024 * 1024 // 10 MB
	html, err := io.ReadAll(io.LimitReader(fetchRes.Body, int64(maxHtmlSize)+1))
	if err != nil {
		app.serverError(w, err)
		return
	}
	if len(html) > maxHtmlSize {
		app.clientError(w, http.StatusRequestEntityTooLarge, "HTML is too large")
		return
	}

	readabilityReqBody, err := json.Marshal(map[string]string{"url": req.URL, "html": string(html)})
	if err != nil {
		app.serverError(w, err)
		return
	}
	readabilityReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, app.readabilityServiceURL+"/extract", bytes.NewReader(readabilityReqBody))
	if err != nil {
		app.serverError(w, err)
		return
	}
	readabilityReq.Header.Set("Content-Type", "application/json")

	readabilityRes, err := app.httpClient.Do(readabilityReq)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer readabilityRes.Body.Close()

	if readabilityRes.StatusCode != http.StatusOK {
		app.serverError(w, fmt.Errorf("readability service returned status: %d", readabilityRes.StatusCode))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	_, err = io.Copy(w, readabilityRes.Body)
	if err != nil {
		app.logger.Error(err.Error())
	}
}
