//go:build webapp

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-openapi/runtime/middleware"
)

//go:embed dist/*
var App embed.FS

//go:embed swagger.yaml
var swaggerSpec embed.FS

func OctantUIHandler() (http.Handler, error) {
	octantApp, err := fs.Sub(App, "dist")
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServerFS(octantApp)
	octantAppHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// If the user is asking for the root, let the file server handle index.html natively
		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the requested file exists in the embedded dist folder
		if _, err = fs.Stat(octantApp, path); err != nil {
			// If the file does NOT exist (like /dashboard), it's a React route!
			// Rewrite the request path to root so FileServerFS serves index.html
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
	return octantAppHandler, nil
}

// withFrameOptions wraps an http.Handler to set headers that prevent iframe embedding (clickjacking protection).
func withFrameOptions(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.ServeHTTP(w, r)
	})
}

// ServeSwaggerUI serves the Swagger UI and JSON spec.
func ServeSwaggerUI(mux *http.ServeMux) {
	mux.Handle("/swagger.yaml", withFrameOptions(http.FileServer(http.FS(swaggerSpec))))
	mux.Handle("/docs/", withFrameOptions(middleware.Redoc(middleware.RedocOpts{
		BasePath: "/",
		SpecURL:  "/swagger.yaml",
		Path:     "docs",
		Title:    "Octant API Docs",
	}, http.NotFoundHandler())))
}
