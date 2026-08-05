package httpapi

import (
	"net/http"
	"sort"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/go-chi/chi/v5"
)

// ContractRoute describes an HTTP route registered by the Go runtime.
type ContractRoute struct {
	Method string
	Path   string
	Source string
}

// ContractRoutes walks the same API route registration methods used by the
// server so generated contracts cannot depend on the retired Java backend.
func ContractRoutes() ([]ContractRoute, error) {
	router := chi.NewRouter()
	noop := func(http.ResponseWriter, *http.Request) {}
	for _, path := range []string{
		"/actuator/health",
		"/actuator/health/liveness",
		"/actuator/health/readiness",
		"/actuator/prometheus",
		"/actuator/metrics",
		"/actuator/metrics/{metricName}",
		"/actuator/info",
	} {
		router.Get(path, noop)
	}

	roomAPI := &RoomAPI{authService: new(account.Service)}
	apis := []interface{ Routes(chi.Router) }{
		&PlatformAPI{},
		&AuthAPI{},
		roomAPI,
		&PlaylistAPI{},
		&AdminAPI{accounts: new(account.Service)},
		&MediaAPI{},
		&WebSocketAPI{},
		&StaticAPI{},
	}
	for _, api := range apis {
		api.Routes(router)
	}

	routes := []ContractRoute{}
	err := chi.Walk(router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, ContractRoute{Method: method, Path: path, Source: "backend-go/internal/httpapi"})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}
