package main

import (
	"embed"
	_ "embed"
)

//go:embed static/dist
var embededFiles embed.FS

//go:embed static/dist.json
var embededfilehash []byte
