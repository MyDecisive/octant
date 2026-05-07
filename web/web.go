//go:build webapp
// +build webapp

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var App embed.FS

func Handler() (http.Handler, error) {
	octantApp, err := fs.Sub(App, "dist")
	if err != nil {
		return nil, err
	}

	return http.FileServerFS(octantApp), nil
}
