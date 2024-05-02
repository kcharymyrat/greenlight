package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/kcharymyrat/greenlight/internal/data"
)

// mux.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.showMovieHandler)
// mux.HandlerFunc(http.MethodPost, "/v1/movies", app.createMovieHandler)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Post: createMovies at %v", r.Body)
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIdParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
	}

	movie := data.Movie{
		ID:        id,
		CreatedAt: time.Now(),
		Title:     "Casablanca",
		Runtime:   102,
		Genres:    []string{"drama", "romance", "war"},
		Version:   1,
	}

	fmt.Println("movie =", movie)

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
