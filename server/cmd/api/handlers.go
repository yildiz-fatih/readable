package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
	"uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (app *application) uploadAndPresign(ctx context.Context, body []byte, contentType, ext string) (string, error) {
	key := uuid.New().String()

	_, err := app.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(app.s3BucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}

	presignedReq, err := app.s3PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(app.s3BucketName),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(fmt.Sprintf(`inline; filename="%s.%s"`, key, ext)),
	}, s3.WithPresignExpires(24*time.Hour))
	if err != nil {
		return "", err
	}

	return presignedReq.URL, nil
}

func (app *application) createReadableHandler(w http.ResponseWriter, r *http.Request) {
	type createReadableRequest struct {
		URL    string `json:"url"`
		Format string `json:"format"` // "html", "pdf", "epub"
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

	if req.Format != "html" && req.Format != "pdf" && req.Format != "epub" {
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

	var link string

	switch req.Format {
	case "html":
		htmlBytes, err := io.ReadAll(readabilityRes.Body)
		if err != nil {
			app.serverError(w, err)
			return
		}
		link, err = app.uploadAndPresign(r.Context(), htmlBytes, "text/html", "html")
		if err != nil {
			app.serverError(w, err)
			return
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
		pdfBytes, err := io.ReadAll(gotenbergRes.Body)
		if err != nil {
			app.serverError(w, err)
			return
		}
		link, err = app.uploadAndPresign(r.Context(), pdfBytes, "application/pdf", "pdf")
		if err != nil {
			app.serverError(w, err)
			return
		}
	case "epub":
		epubServiceReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, app.epubServiceURL+"/html-to-epub", readabilityRes.Body)
		if err != nil {
			app.serverError(w, err)
			return
		}
		epubServiceReq.Header.Set("Content-Type", "text/html")
		epubServiceRes, err := app.httpClient.Do(epubServiceReq)
		if err != nil {
			app.serverError(w, err)
			return
		}
		defer epubServiceRes.Body.Close()
		if epubServiceRes.StatusCode != http.StatusOK {
			app.serverError(w, fmt.Errorf("epub service returned status: %d", epubServiceRes.StatusCode))
			return
		}
		epubBytes, err := io.ReadAll(epubServiceRes.Body)
		if err != nil {
			app.serverError(w, err)
			return
		}
		link, err = app.uploadAndPresign(r.Context(), epubBytes, "application/epub+zip", "epub")
		if err != nil {
			app.serverError(w, err)
			return
		}
	}

	err = writeJSON(w, http.StatusOK, nil, map[string]string{
		"link": link,
	})
	if err != nil {
		app.serverError(w, err)
		return
	}
}
