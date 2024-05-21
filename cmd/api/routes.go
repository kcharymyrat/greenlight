package main

import (
	"expvar"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *application) routes() http.Handler {
	// initialize new router (mux)
	mux := chi.NewRouter()

	mux.NotFound(app.notFoundResponse)
	mux.MethodNotAllowed(app.methodNotAllowedResponse)

	// Map the appropriate handler for the request based on the request path
	mux.Get("/v1/healthcheck", app.healthcheckHandler)

	mux.Get("/v1/movies", app.requirePermission("movies:read", app.listMoviesHandler))
	mux.Post("/v1/movies", app.requirePermission("movies:write", app.createMovieHandler))
	mux.Get("/v1/movies/{id}", app.requirePermission("movies:read", app.showMovieHandler))
	mux.Patch("/v1/movies/{id}", app.requirePermission("movies:write", app.updateMovieHandler))
	mux.Delete("/v1/movies/{id}", app.requirePermission("movies:write", app.deleteMovieHandler))

	mux.Post("/v1/users", app.registerUserHandler)
	mux.Put("/v1/users/activated", app.activateUserHandler)

	mux.Post("/v1/tokens/authentication", app.createAuthenticationTokenHandler)

	// mux.Get("/debug/vars", expvar.Handler().ServeHTTP)
	mux.Get("/debug/vars", app.requirePermission("metrics:view", expvar.Handler().ServeHTTP))

	return app.metrics(app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(mux)))))
}
