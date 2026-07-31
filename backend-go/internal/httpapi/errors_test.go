package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMappedErrorsMatchJavaShapes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		want       map[string]any
	}{
		{name: "upstream", err: BadGateway("upstream failed"), wantStatus: 502, want: map[string]any{"message": "upstream failed", "status": float64(502)}},
		{name: "upload", err: UploadTooLarge("MaxUploadSizeExceededException"), wantStatus: 413, want: map[string]any{"message": uploadTooLargeMessage, "error": "MaxUploadSizeExceededException", "status": float64(413)}},
		{name: "generic", err: errors.New("boom"), wantStatus: 500, want: map[string]any{"message": "An unexpected internal server error occurred.", "error": "InternalServerError", "status": float64(500)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			WriteMappedError(response, test.err)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(body) != len(test.want) {
				t.Fatalf("body = %#v, want %#v", body, test.want)
			}
			for key, want := range test.want {
				if body[key] != want {
					t.Fatalf("body[%q] = %#v, want %#v", key, body[key], want)
				}
			}
		})
	}
}

func TestAdaptWritesMappedError(t *testing.T) {
	handler := Adapt(func(http.ResponseWriter, *http.Request) error { return BadGateway("failed") })
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", response.Code)
	}
}
