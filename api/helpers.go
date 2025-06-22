package api

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"net/http"
)

var (
	ErrInternal = errors.New("internal error")
)

// logs an error if it's not nil
func logError(err error) {
	if err != nil {
		log.Printf("error: %v", err)
	}
}

// writeJSON writes the data as json.
func writeJSON(w http.ResponseWriter, data interface{}, statusCode int) error {
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(data)
	return err
}

// writeError writes the message as an error
func writeError(w http.ResponseWriter, message string, statusCode int) error {
	log.Printf("response errored: %s", message)
	return writeJSON(
		w,
		map[string]string{"error": message},
		statusCode,
	)
}

// extractAPIVersion extracts the API version from the request path.
func extractAPIVersion(path string) string {
    parts := strings.Split(path, "/")
    for i, part := range parts {
        if part == "api" && i+1 < len(parts) {
            return parts[i+1]
        }
    }
    return ""
}