package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/frankie-mur/greenlight/internal/data"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "creating a new movie")
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	//Retrieve the path param from the request
	id, err := app.readIdParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	movie := data.Movie{
		ID:        id,
		CreatedAt: time.Now(),
		Title:     "Jaws",
		Runtime:   120,
		Genres:    []string{"horror"},
		Version:   1,
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
