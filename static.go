package main

import (
	"embed"
	_ "embed"
)

//go:embed static/dist
var embededFiles embed.FS
