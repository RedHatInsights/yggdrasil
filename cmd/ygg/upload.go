package main

import (
	"encoding/json"
	"os"

	yggdrasil "github.com/redhatinsights/yggdrasil"
)

func upload(client *yggdrasil.HTTPClient, filePath string, collector, metadataPath string) (string, error) {
	var metadata yggdrasil.CanonicalFacts
	if metadataPath != "" {
		f, err := os.Open(metadataPath)
		if err != nil {
			return "", err
		}
		defer f.Close()

		decoder := json.NewDecoder(f)
		if err := decoder.Decode(&metadata); err != nil {
			return "", err
		}
	}

	return yggdrasil.Upload(client, filePath, collector, &metadata)
}
