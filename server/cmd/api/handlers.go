package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

func (app *application) createReadableHandler(w http.ResponseWriter, r *http.Request) {
	type createReadableRequest struct {
		URL    string `json:"url"`
		Format string `json:"format"` // "html", "pdf"
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

	if req.Format != "pdf" && req.Format != "html" {
		app.clientError(w, http.StatusBadRequest, "unsupported format")
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

	switch req.Format {
	case "html":
		w.Header().Set("Content-Type", "text/html")
		_, err = io.Copy(w, readabilityRes.Body)
		if err != nil {
			app.logger.Error(err.Error())
		}
	case "pdf":
		var buf bytes.Buffer
		multipartWriter := multipart.NewWriter(&buf)
		form, err := multipartWriter.CreateFormFile("files", "index.html")
		if err != nil {
			app.serverError(w, err)
			return
		}
		_, err = io.Copy(form, readabilityRes.Body)
		if err != nil {
			app.serverError(w, err)
			return
		}
		err = multipartWriter.Close()
		if err != nil {
			app.serverError(w, err)
			return
		}
		gotenbergReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, app.gotenbergURL+"/forms/chromium/convert/html", &buf)
		if err != nil {
			app.serverError(w, err)
			return
		}
		gotenbergReq.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		gotenbergRes, err := app.httpClient.Do(gotenbergReq)
		if err != nil {
			app.serverError(w, err)
			return
		}
		defer gotenbergRes.Body.Close()
		if gotenbergRes.StatusCode != http.StatusOK {
			app.serverError(w, fmt.Errorf("gotenberg returned status: %d", gotenbergRes.StatusCode))
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, err = io.Copy(w, gotenbergRes.Body)
		if err != nil {
			app.logger.Error(err.Error())
		}
	}

}
