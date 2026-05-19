//go:build webapp

package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-openapi/runtime/middleware"
)

//go:embed dist/*
var App embed.FS

//go:embed swagger.json
var swaggerSpec embed.FS

func OctantUIHandler() (http.Handler, error) {
	octantApp, err := fs.Sub(App, "dist")
	if err != nil {
		return nil, err
	}

	return http.FileServerFS(octantApp), nil
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
	mux.Handle("/swagger.json", withFrameOptions(http.FileServer(http.FS(swaggerSpec))))
	mux.Handle("/docs/", withFrameOptions(middleware.Redoc(middleware.RedocOpts{
		BasePath: "/",
		SpecURL:  "/swagger.json",
		Path:     "docs",
		Title:    "Octant API Docs",
	}, http.NotFoundHandler())))
}
