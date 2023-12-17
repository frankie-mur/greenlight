package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/frankie-mur/greenlight/internal/data"
	"golang.org/x/time/rate"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a deferred function (which will always be run in the event of a panic
		// as Go unwinds the stack).
		defer func() {
			// Use the builtin recover function to check if there has been a panic or
			// not.
			if err := recover(); err != nil {
				// If there was a panic, set a "Connection: close" header on the
				// response. This acts as a trigger to make Go's HTTP server
				// automatically close the current connection after a response has been
				// sent.
				w.Header().Set("Connection", "close")
				// The value returned by recover() has the type any, so we use
				// fmt.Errorf() to normalize it into an error and call our
				// serverErrorResponse() helper. In turn, this will log the error using
				// our custom Logger type at the ERROR level and send the client a 500
				// Internal Server Error response.
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	//Rate limiter is IP-Based to do this we create a mutex (maps are no concurrent safe)
	//and a map of clients
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	//Go routine to remove stale client entries
	go func() {
		for {
			time.Sleep(time.Minute)

			// Lock the mutex to prevent any rate limiter checks from happening while
			// the cleanup is taking place.
			mu.Lock()

			// Loop through all clients. If they haven't been seen within the last three
			// minutes, delete the corresponding entry from the map.
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}

			// Importantly, unlock the mutex when the cleanup is complete.
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.config.limiter.enabled {
			//Extraxt clients IP from request
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}
			// Lock the mutex to prevent this code from being executed concurrently.
			mu.Lock()

			// Check to see if the IP address already exists in the map. If it doesn't, then
			// initialize a new rate limiter and add the IP address and limiter to the map.
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst)}
			}

			// Update the last seen time for the client.
			clients[ip].lastSeen = time.Now()

			// Call the Allow() method on the rate limiter for the current IP address. If
			// the request isn't allowed, unlock the mutex and send a 429 Too Many Requests
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}
			//unlock the mutex before calling the next handler in the
			// chain. Notice that we DON'T use defer to unlock the mutex, as that would mean
			// that the mutex isn't unlocked until all the handlers downstream of this
			// middleware have also returned.
			mu.Unlock()

			next.ServeHTTP(w, r)
		}

		next.ServeHTTP(w, r)
	})
}

// Middleware is used to authenticate an auth token in headers
func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//Set Vary header for any cache
		r.Header.Set("Vary", "Authorization")

		//Retrieve auth header from request
		authHeader := r.Header.Get("Authorization")

		//Check if exists
		if len(authHeader) == 0 {
			//If not set to anonymous user
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		//Validate the authorization header
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		//extract the token
		token := headerParts[1]

		//retrieve the user matching the given token
		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
		//Add user to the request context
		r = app.contextSetUser(r, user)

		next.ServeHTTP(w, r)
	})
}

// Middleware checks that request i an Authenticated User (not anonymous)
func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Checks that a user is both authenticated and activated
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//Get the user from the request context
		user := app.contextGetUser(r)

		// Check that the user is activated to proceed with the request
		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		//User is not annoymous and is activated therefore we can call next handler
		next.ServeHTTP(w, r)
	})
	//Wrap in require authentication middleware to check that authenticated
	return app.requireAuthenticatedUser(fn)
}

func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		//Get the user from the request context
		user := app.contextGetUser(r)

		//Get the permissions of the user
		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		//Check that user has required permissions
		if !permissions.Include(code) {
			app.notPermittedResponse(w, r)
			return
		}

		//User has required permissions
		next.ServeHTTP(w, r)
	}

	return app.requireActivatedUser(fn)
}

// Middleware to set header that enables CORS
func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")

		w.Header().Add("Vary", "Access-Control-Request-Method")

		reqOrigin := r.Header.Get("Origin")
		//If the request origin is in our list of trusted origins
		// Set CORS header and allow
		if len(reqOrigin) > 0 {
			for _, val := range app.config.cors.trustedOrigins {
				if reqOrigin == val {
					r.Header.Set("Access-Control-Allow-Origin", reqOrigin)

					// Check if the request has the HTTP method OPTIONS and contains the
					// "Access-Control-Request-Method" header. If it does, then we treat
					// it as a preflight request.
					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						// Set the necessary preflight response headers
						w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

						// Write the headers along with a 200 OK status and return from
						// the middleware with no further action.
						w.WriteHeader(http.StatusOK)
						return
					}
				}
			}
		}

		//Else do nothing and continue
		next.ServeHTTP(w, r)
	})
}
