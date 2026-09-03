package main

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, statusCode int, headers http.Header, body any) error {
	js, err := json.Marshal(body)
	if err != nil {
		return err
	}

	for key, values := range headers {
		w.Header()[key] = values
	}
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	w.Write(js)
	return nil
}

func (app *application) serverError(w http.ResponseWriter, err error) {
	app.logger.Error(err.Error())

	jsonErr := writeJSON(w, http.StatusInternalServerError, nil, map[string]string{
		"error": http.StatusText(http.StatusInternalServerError),
	})
	if jsonErr != nil {
		app.logger.Error(jsonErr.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (app *application) clientError(w http.ResponseWriter, statusCode int, message string) {
	err := writeJSON(w, statusCode, nil, map[string]string{
		"error": message,
	})
	if err != nil {
		app.logger.Error(err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
