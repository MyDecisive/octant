//go:build webapp
// +build webapp

package web

import "embed"

//go:embed dist/*
var App embed.FS
