// Package env is a tiny helper for 12-factor style configuration.
//
// Services read ports and endpoints from the environment so the same binary
// works under make demo (localhost), Docker Compose (service DNS names), and
// Kubernetes (cluster DNS + ConfigMap). No simulation logic lives here.
package env

import (
	"os"
	"strconv"
)

// Get returns the environment variable or a default.
func Get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// GetInt returns an int environment variable or a default.
func GetInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
