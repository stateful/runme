package server

import (
	"net/http"

	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"

	"github.com/runmedev/runme/v3/pkg/agent/logs"
)

// SetOriginHeader is middleware that copies the Origin header from the request to the response
// This is necessary when using AllowAllOrigins because the browser will complain if the response header
// is the "*" and not the same origin as on the request. The cors handler in the connect library doesn't do
// this by default.
func SetOriginHeader(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logs.FromContext(r.Context())
		// Always set the Access-Control-Allow-Origin header to the request's Origin header
		// Not just if its an Options request.
		// It looks like the CORS handler
		// (https://github.com/rs/cors/blob/1084d89a16921942356d1c831fbe523426cf836e/cors.go#L323) checks origin
		// and sets the response headers even if its not an OPTIONS request. So we need to always set the
		// Access-Control-Allow-Origin header to the request's Origin header.
		w.Header()["Access-Control-Allow-Origin"] = r.Header["Origin"]
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			log.V(logs.Debug).Info("Setting Access-Control-Allow-Origin header", "origin", r.Header["Origin"])
			// http.StatusNoContent is used for preflight requests
			w.WriteHeader(http.StatusNoContent)
		} else {
			// Shared authentication supports query credentials; never log query values.
			log.V(logs.Debug).Info("Calling next handler", "method", r.Method, "path", r.URL.Path)
			h.ServeHTTP(w, r)
		}
	})
}

func wrapWithCORS(handler http.Handler, origins []string, allowCredentials bool) http.Handler {
	if len(origins) == 0 {
		return handler
	}

	log := logs.NewLogger()
	// This is modeled on cors.AllowAll() but we can't use that because we may need to allow credentials
	corsOptions := cors.Options{
		AllowedOrigins: origins,
		// Allow all methods.
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		// Allow all headers.
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   connectcors.ExposedHeaders(),
		AllowCredentials: allowCredentials,
		MaxAge:           7200, // 2 hours in seconds
	}
	log.Info("Adding CORS support", "AllowedOrigins", corsOptions.AllowedOrigins, "AllowCredentials", corsOptions.AllowCredentials, "AllowedMethods", corsOptions.AllowedMethods, "AllowedHeaders", corsOptions.AllowedHeaders, "ExposedHeaders", corsOptions.ExposedHeaders)

	for _, origin := range origins {
		if origin != "*" {
			continue
		}
		log.Info("Allowing all origins; enabling SetOriginHeader middleware")
		// We need to set the origin header to the request's origin
		// To do that we need to set the passthrough option to true so that the handler will invoke our middleware
		// after calling the cors handler
		corsOptions.OptionsPassthrough = true
		corsOptions.Debug = true
		c := cors.New(corsOptions)
		return c.Handler(SetOriginHeader(handler))
	}

	c := cors.New(corsOptions)
	return c.Handler(handler)
}
