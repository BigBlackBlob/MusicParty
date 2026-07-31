package media

import (
	"io"
	"net/http"
)

// Proxy streams an upstream response without buffering the media body.
type Proxy struct{ Client *http.Client }

func (p Proxy) Serve(w http.ResponseWriter, r *http.Request, upstreamURL string, headers http.Header) error {
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		return err
	}
	for name, values := range headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	if value := r.Header.Get("Range"); value != "" {
		request.Header.Set("Range", value)
	}
	response, err := p.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	if response.StatusCode >= 400 {
		w.WriteHeader(http.StatusBadGateway)
		return nil
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
	w.WriteHeader(response.StatusCode)
	_, err = io.Copy(w, response.Body)
	return err
}
