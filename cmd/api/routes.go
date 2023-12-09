package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()
	//Set custom errors for httprouter NotFound and MethodNotAllowed responses
	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	//health route
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheck)

	//movie routes
	router.HandlerFunc(http.MethodPost, "/v1/movies", app.createMovieHandler)
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.showMovieHandler)
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.updateMovieHandler)
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.deleteMovieHandler)
	router.HandlerFunc(http.MethodGet, "/v1/movies", app.listMovieHandler)

	//user routes
	router.HandlerFunc(http.MethodPost, "/v1/users", app.createUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)

	//token routes
	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthTokenHandler)

	return app.recoverPanic(app.rateLimit(app.authenticate(router)))
}
