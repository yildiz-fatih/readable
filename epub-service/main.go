package main

import (
	"bytes"
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

	mux.HandleFunc("POST /html-to-epub", app.htmlToEpubHandler)

	logger.Info("starting epub service", "address", ":"+port)
	err := http.ListenAndServe(":"+port, mux)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func (app *application) htmlToEpubHandler(w http.ResponseWriter, r *http.Request) {
	cmd := exec.CommandContext(r.Context(), "pandoc", "-f", "html", "-t", "epub")
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

	w.Header().Set("Content-Type", "application/epub+zip")
	_, err = w.Write(outBuffer.Bytes())
	if err != nil {
		app.logger.Error("failed to write response", "error", err.Error())
	}
}
