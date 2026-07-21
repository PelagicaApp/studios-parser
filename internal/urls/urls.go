package urls

import (
	"net/url"
	"strings"

	"pelagica-studios/internal/env"
)

const baseURL = "https://api.themoviedb.org"

func BuildURL(path string, queryParams map[string]string) string {
	normalizedPath := path
	if !strings.HasPrefix(path, "/") {
		normalizedPath = "/" + path
	}

	result := baseURL + normalizedPath
	if len(queryParams) > 0 {
		values := url.Values{}
		for key, value := range queryParams {
			values.Set(key, value)
		}
		result += "?" + values.Encode()
	}
	return result
}

type HeaderOptions struct {
	JSON bool
	Auth bool
}

func DefaultHeaderOptions() HeaderOptions {
	return HeaderOptions{JSON: true, Auth: true}
}

func BuildHeaders(options HeaderOptions) (map[string]string, error) {
	headers := map[string]string{}

	if options.JSON {
		headers["Content-Type"] = "application/json"
		headers["Accept"] = "application/json"
	}

	if options.Auth {
		apiKey, err := env.TMDBAPIKey()
		if err != nil {
			return nil, err
		}
		headers["Authorization"] = "Bearer " + apiKey
	}

	return headers, nil
}
