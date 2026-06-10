//go:build webapp

package web

import (
	"embed"
	"io/fs"
	"net/http"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-openapi/runtime/middleware"
)

//go:embed dist/*
var App embed.FS

//go:embed swagger.yaml
var swaggerSpec embed.FS

type OctantApp struct {
	distFiles    mapset.Set[string]
	filesHandler http.Handler
}

func NewOctantApp() (*OctantApp, error) {
	octantApp, err := fs.Sub(App, "dist")
	if err != nil {
		return nil, err
	}

	// technically this could be accessed across threads at the same time, but this will never
	// be modified after we populate it here, so no mutex needed!
	octantWebFiles := mapset.NewThreadUnsafeSet[string]()
	// build up all the files in dist for quick lookup later in the handler.
	if err = fs.WalkDir(octantApp, ".", func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		octantWebFiles.Add(path)
		return nil
	}); err != nil {
		return nil, err
	}

	return &OctantApp{
		distFiles:    octantWebFiles,
		filesHandler: http.FileServerFS(octantApp),
	}, nil
}

func (oa *OctantApp) UIHandler() http.Handler {
	octantAppHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// If the user is asking for the root, let the file server handle index.html natively
		if path == "" {
			oa.filesHandler.ServeHTTP(w, r)
			return
		}

		// Check if the requested file exists in the embedded dist folder
		if !oa.distFiles.Contains(path) {
			// If the file does NOT exist (like /dashboard), it's a React route!
			// Rewrite the request path to root so FileServerFS serves index.html
			r.URL.Path = "/"
		}
		oa.filesHandler.ServeHTTP(w, r)
	})
	return octantAppHandler
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
