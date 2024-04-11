package main

import (
	"embed"
	"encoding/json"
	// _ "embed"
)

//go:embed static/dist
var embededFiles embed.FS

const staticfileprefix = "static/dist"

//go:embed static/dist.json
var embededfilehash []byte

func byteToJSONMap(data []byte) (map[string]string, error) {
	var jsonMap map[string]string
	err := json.Unmarshal(data, &jsonMap)
	if err != nil {
		return nil, err
	}
	return jsonMap, nil
}
