package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (app *application) newRouter() http.Handler {
	rtr := chi.NewRouter()

	rtr.Use(middleware.Recoverer)

	rtr.Use(middleware.Heartbeat("/ping"))

	rtr.Post("/readables", app.createReadableHandler)

	return rtr
}
