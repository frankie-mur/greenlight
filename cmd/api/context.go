package main

import (
	"context"
	"net/http"

	"github.com/frankie-mur/greenlight/internal/data"
)

type contextKey string

const userContextKey = contextKey("user")

// Set the request context to with a key user and the provided user
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

// Given a request get the user data from the context
func (app *application) contextGetUser(r *http.Request) *data.User {
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user value in request context")
	}

	return user
}
