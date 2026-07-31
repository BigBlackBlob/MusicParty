package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
)

const uploadTooLargeMessage = "Upload is too large. Increase MULTIPART_MAX_FILE_SIZE / MULTIPART_MAX_REQUEST_SIZE or choose a smaller file."

type APIError struct {
	Status  int
	Message string
	Name    string
}

func (e *APIError) Error() string { return e.Message }

func BadGateway(message string) error {
	return &APIError{Status: http.StatusBadGateway, Message: message}
}

func UploadTooLarge(name string) error {
	return &APIError{Status: http.StatusRequestEntityTooLarge, Message: uploadTooLargeMessage, Name: name}
}

type ErrorHandler func(http.ResponseWriter, *http.Request) error

func Adapt(handler ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			WriteMappedError(w, err)
		}
	}
}

func WriteMappedError(w http.ResponseWriter, err error) {
	var apiError *APIError
	if errors.As(err, &apiError) {
		body := map[string]any{"message": apiError.Message, "status": apiError.Status}
		if apiError.Name != "" {
			body["error"] = apiError.Name
		}
		writeErrorJSON(w, apiError.Status, body)
		return
	}
	writeErrorJSON(w, http.StatusInternalServerError, map[string]any{
		"message": "An unexpected internal server error occurred.",
		"error":   "InternalServerError",
		"status":  http.StatusInternalServerError,
	})
}

func writeErrorJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
