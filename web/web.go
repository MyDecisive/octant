//go:build webapp

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var App embed.FS

//go:embed swagger/*
//go:embed octant_docs.swagger.json
var SwaggerUI embed.FS

func OctantUIHandler() (http.Handler, error) {
	octantApp, err := fs.Sub(App, "dist")
	if err != nil {
		return nil, err
	}

	return http.FileServerFS(octantApp), nil
}

func SwaggerUIHandler() (http.Handler, error) {
	swaggerUI, err := fs.Sub(SwaggerUI, "swagger")
	if err != nil {
		return nil, err
	}

	return http.FileServer(http.FS(swaggerUI)), nil
}
