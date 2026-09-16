package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (app *application) newRouter() http.Handler {
	rtr := chi.NewRouter()

	rtr.Use(middleware.Recoverer)

	rtr.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{app.frontendURL},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
	}))

	rtr.Use(middleware.Heartbeat("/ping"))

	rtr.Post("/readables", app.createReadableHandler)
	rtr.Get("/readables/{id}", app.getReadableHandler)

	return rtr
}
