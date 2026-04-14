package web

import "embed"

//go:embed dist/*
var App embed.FS
