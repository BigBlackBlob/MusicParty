package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/coder/websocket"
)

type Envelope struct {
	Type      string `json:"type"`
	RequestID any    `json:"requestId"`
	RoomID    any    `json:"roomId"`
	Payload   any    `json:"payload"`
}

func ReadJSON(ctx context.Context, connection *websocket.Conn, target any) error {
	connection.SetReadLimit(1 << 20)
	_, data, err := connection.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func Accept(w http.ResponseWriter, r *http.Request, allowedOrigins []string) (*websocket.Conn, error) {
	if !OriginAllowed(r, allowedOrigins) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return nil, errors.New("websocket origin is not allowed")
	}
	return websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		CompressionMode:    websocket.CompressionContextTakeover,
	})
}

func OriginAllowed(r *http.Request, allowedOrigins []string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	requestScheme := "http"
	if r.TLS != nil {
		requestScheme = "https"
	}
	if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		requestScheme = strings.TrimSpace(strings.SplitN(forwardedProto, ",", 2)[0])
	}
	if strings.EqualFold(origin, requestScheme+"://"+r.Host) {
		return true
	}
	for _, allowed := range allowedOrigins {
		if origin == strings.TrimSpace(allowed) {
			return true
		}
	}
	return false
}
