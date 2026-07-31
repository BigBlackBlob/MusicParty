package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/go-chi/chi/v5"
)

const requestIDHeader = "X-Request-Id"

type clientIPContextKey struct{}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(value []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(value)
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" || len(requestID) > 128 {
			var value [16]byte
			if _, err := rand.Read(value[:]); err == nil {
				requestID = hex.EncodeToString(value[:])
			} else {
				requestID = strconv.FormatInt(time.Now().UnixNano(), 36)
			}
		}
		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r)
	})
}

func TrustedProxyClientIP(trustedCIDRs []string) func(http.Handler) http.Handler {
	trusted := make([]*net.IPNet, 0, len(trustedCIDRs))
	for _, cidr := range trustedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			trusted = append(trusted, network)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			remoteAddress := parseRemoteAddress(r.RemoteAddr)
			clientIP := remoteAddress
			if isTrusted(net.ParseIP(remoteAddress), trusted) {
				forwardedFound := false
				forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
				for _, raw := range forwarded {
					candidate := cleanAddress(raw)
					if candidate == "" {
						continue
					}
					clientIP = candidate
					forwardedFound = true
					break
				}
				if !forwardedFound {
					if candidate := cleanAddress(r.Header.Get("X-Real-IP")); candidate != "" {
						clientIP = candidate
					}
				}
			}
			ctx := context.WithValue(r.Context(), clientIPContextKey{}, clientIP)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClientIP(r *http.Request) string {
	if value, ok := r.Context().Value(clientIPContextKey{}).(string); ok {
		return value
	}
	return parseRemoteAddress(r.RemoteAddr)
}

func parseRemoteAddress(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	return cleanAddress(host)
}

func cleanAddress(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.EqualFold(trimmed, "unknown") {
		return ""
	}
	if zone := strings.IndexByte(trimmed, '%'); zone >= 0 {
		return trimmed[:zone]
	}
	return trimmed
}

func isTrusted(ip net.IP, networks []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("http handler panic", "panic", recovered, "method", r.Method, "path", r.URL.Path)
					WriteMappedError(w, errors.New("panic"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func AccessLog(logger *slog.Logger, metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapped, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "UNKNOWN"
			}
			outcome := "SUCCESS"
			if wrapped.status >= 500 {
				outcome = "SERVER_ERROR"
			} else if wrapped.status >= 400 {
				outcome = "CLIENT_ERROR"
			}
			status := strconv.Itoa(wrapped.status)
			metrics.HTTPRequests.WithLabelValues(r.Method, route, status, outcome).Inc()
			metrics.HTTPDuration.WithLabelValues(r.Method, route, status, outcome).Observe(time.Since(startedAt).Seconds())
			logger.Info("http request", "requestId", w.Header().Get(requestIDHeader), "clientIp", ClientIP(r), "method", r.Method, "path", r.URL.Path, "status", wrapped.status, "durationMs", time.Since(startedAt).Milliseconds())
		})
	}
}

func CORS(_ []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/media/") {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("Origin") != "" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func CSRF(exemptPaths map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if _, exempt := exemptPaths[r.URL.Path]; exempt {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie("MP_CSRF")
			header := r.Header.Get("X-CSRF-Token")
			if err != nil || cookie.Value == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
