package main

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"

	"github.com/joho/godotenv"
)

type application struct {
	logger *slog.Logger
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load("../.env")

	port := os.Getenv("EPUB_SERVICE_PORT")
	if port == "" {
		port = "8082"
	}

	app := &application{
		logger: logger,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /convert", app.convertHandler)

	logger.Info("starting epub service", "address", ":"+port)
	err := http.ListenAndServe(":"+port, mux)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

/*
* /convert?to=epub
* /convert?to=md
 */
func (app *application) convertHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()

	to := queryParams.Get("to")
	if to == "" || (to != "epub" && to != "md") {
		err := errors.New("query parameter 'to' is missing or invalid. try '?to=epub' or '?to=md'")
		app.logger.Error(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var toPandocFlag string
	var toMimeType string

	switch to {
	case "epub":
		toPandocFlag = "epub"
		toMimeType = "application/epub+zip"
	case "md":
		toPandocFlag = "gfm-raw_html"
		toMimeType = "text/markdown"
	}

	cmd := exec.CommandContext(r.Context(), "pandoc", "-f", "html", "-t", toPandocFlag)
	cmd.Stdin = r.Body
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	var errBuffer bytes.Buffer
	cmd.Stderr = &errBuffer

	err := cmd.Run()
	if err != nil {
		app.logger.Error("pandoc conversion failed", "error", err.Error(), "stderr", errBuffer.String())
		http.Error(w, errBuffer.String(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", toMimeType)

	_, err = w.Write(outBuffer.Bytes())
	if err != nil {
		app.logger.Error("failed to write response", "error", err.Error())
	}
}
