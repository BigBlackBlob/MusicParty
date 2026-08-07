package contractspec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/httpapi"
)

type GeneratedFile struct {
	Path    string
	Content []byte
}

type route struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Operation string `json:"operationId"`
	Source    string `json:"source"`
}

type environmentVariable struct {
	Name        string `json:"name"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Deprecated  bool   `json:"deprecated"`
	Description string `json:"description"`
}

var configCallPattern = regexp.MustCompile(`r\.(?:string|bool|int|int64|durationMS|durationSeconds|list)\("([A-Z0-9_]+)"(?:,\s*([^\)\r\n]+))?\)`)

func Generate(repositoryRoot string) ([]GeneratedFile, error) {
	runtimeRoutes, err := httpapi.ContractRoutes()
	if err != nil {
		return nil, fmt.Errorf("inspect Go HTTP routes: %w", err)
	}
	routes := make([]route, 0, len(runtimeRoutes))
	for _, item := range runtimeRoutes {
		routes = append(routes, route{
			Method: item.Method, Path: item.Path,
			Operation: operationID(item.Method, item.Path), Source: item.Source,
		})
	}
	environment, err := inspectGoEnvironment(repositoryRoot)
	if err != nil {
		return nil, err
	}
	client, server := clientMessageSpecs(), serverMessageSpecs()
	assets := []GeneratedFile{
		jsonGeneratedFile("contracts/http/routes.json", map[string]any{"version": 2, "runtime": "go", "routeCount": len(routes), "routes": routes}),
		jsonGeneratedFile("contracts/http/openapi.yaml", openAPI(routes)),
		jsonGeneratedFile("contracts/http/golden/scenarios.json", goHTTPScenarios()),
		jsonGeneratedFile("contracts/ws/client-messages.schema.json", websocketSchema("MusicParty Go WebSocket client messages", client)),
		jsonGeneratedFile("contracts/ws/server-messages.schema.json", websocketSchema("MusicParty Go WebSocket server messages", server)),
		jsonGeneratedFile("contracts/ws/scenarios/catalog.json", goWebSocketScenarios()),
		jsonGeneratedFile("contracts/config/environment.yaml", map[string]any{"version": 2, "runtime": "go", "variables": environment}),
	}
	return append(assets, goFrontendFiles()...), nil
}

func Write(repositoryRoot string, files []GeneratedFile) error {
	for _, file := range files {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create contract directory: %w", err)
		}
		if err := os.WriteFile(path, file.Content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", file.Path, err)
		}
	}
	return nil
}

func Check(repositoryRoot string, files []GeneratedFile) error {
	for _, file := range files {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(file.Path))
		actual, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read generated contract %s: %w", file.Path, err)
		}
		if !bytes.Equal(actual, file.Content) {
			return fmt.Errorf("generated contract is stale: %s", file.Path)
		}
	}
	return nil
}

func operationID(method, path string) string {
	value := strings.Trim(path, "/")
	value = strings.NewReplacer("/", ".", "{", "", "}", "", "-", "_").Replace(value)
	if value == "" {
		value = "root"
	}
	return strings.ToLower(method) + "." + value
}

func openAPI(routes []route) map[string]any {
	paths := map[string]any{}
	for _, item := range routes {
		pathItem, _ := paths[item.Path].(map[string]any)
		if pathItem == nil {
			pathItem = map[string]any{}
			paths[item.Path] = pathItem
		}
		pathItem[strings.ToLower(item.Method)] = map[string]any{
			"operationId": item.Operation,
			"x-go-source": item.Source,
			"responses": map[string]any{
				"200":     map[string]any{"description": "Success"},
				"default": map[string]any{"$ref": "#/components/responses/Error"},
			},
		}
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "MusicParty Go HTTP API", "version": frontendContractVersion},
		"paths":   paths,
		"components": map[string]any{
			"schemas": map[string]any{
				"APIError": map[string]any{
					"type": "object", "additionalProperties": false,
					"required": []string{"status", "code", "message"},
					"properties": map[string]any{
						"status":      map[string]any{"type": "integer"},
						"code":        map[string]any{"type": "string"},
						"message":     map[string]any{"type": "string"},
						"requestId":   map[string]any{"type": "string"},
						"fieldErrors": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
					},
				},
			},
			"responses": map[string]any{
				"Error": map[string]any{
					"description": "API error",
					"content":     map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/APIError"}}},
				},
			},
		},
	}
}

func inspectGoEnvironment(repositoryRoot string) ([]environmentVariable, error) {
	path := filepath.Join(repositoryRoot, "backend-go", "internal", "config", "config.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Go config: %w", err)
	}
	byName := map[string]environmentVariable{}
	for _, match := range configCallPattern.FindAllStringSubmatch(string(content), -1) {
		name := match[1]
		fallback := strings.TrimSpace(match[2])
		fallback = strings.Trim(fallback, `"`)
		upper := strings.ToUpper(name)
		secret := strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "COOKIE") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "SESSDATA") || strings.Contains(upper, "API_KEY")
		deprecated := name == "APP_ENV" || name == "ENV" || name == "NODE_ENV" || name == "SPRING_PROFILES_ACTIVE" || name == "ADMIN_PASSWORD"
		byName[name] = environmentVariable{
			Name: name, Default: fallback, Required: false, Secret: secret, Deprecated: deprecated,
			Description: "MusicParty Go runtime configuration",
		}
	}
	variables := make([]environmentVariable, 0, len(byName))
	for _, item := range byName {
		variables = append(variables, item)
	}
	sort.Slice(variables, func(i, j int) bool { return variables[i].Name < variables[j].Name })
	return variables, nil
}

func goHTTPScenarios() map[string]any {
	return map[string]any{
		"version": 2, "runtime": "go",
		"scenarios": []any{
			map[string]any{"id": "health", "method": "GET", "path": "/actuator/health", "expectedStatus": 200},
			map[string]any{"id": "account-status", "method": "GET", "path": "/api/account/status", "expectedStatus": 200},
			map[string]any{"id": "account-me-unauthenticated", "method": "GET", "path": "/api/account/me", "expectedStatus": 401},
			map[string]any{"id": "rooms-list", "method": "GET", "path": "/api/rooms", "expectedStatus": 200},
		},
	}
}

func goWebSocketScenarios() map[string]any {
	return map[string]any{
		"version": 2, "runtime": "go", "endpoint": "/ws", "cookie": "MP_SESSION",
		"clientMessageTypes": messageNames(clientMessageSpecs()),
		"serverMessageTypes": messageNames(serverMessageSpecs()),
		"scenarios": []any{
			map[string]any{"id": "unauthorized-handshake", "expectCloseCode": 1008},
			map[string]any{"id": "resync", "sendType": "player.resync", "expectTypes": []string{"player.state"}},
			map[string]any{"id": "ping", "sendType": "sync.ping", "expectTypes": []string{"sync.pong"}},
			map[string]any{"id": "presence", "sendType": "users.online", "expectTypes": []string{"users.online"}},
		},
	}
}

func messageNames(messages []messageSpec) []string {
	result := make([]string, 0, len(messages))
	for _, message := range messages {
		result = append(result, message.Name)
	}
	sort.Strings(result)
	return result
}
