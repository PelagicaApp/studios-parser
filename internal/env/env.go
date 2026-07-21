package env

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	_ = godotenv.Load()
}

func RequireEnvVar(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is not set", name)
	}
	return value, nil
}

func TMDBAPIKey() (string, error) {
	return RequireEnvVar("TMDB_API_KEY")
}
