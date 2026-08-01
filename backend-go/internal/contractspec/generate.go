package contractspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type GeneratedFile struct {
	Path    string
	Content []byte
}

type route struct {
	Method              string           `json:"method"`
	Path                string           `json:"path"`
	Operation           string           `json:"operationId"`
	Controller          string           `json:"controller"`
	Source              string           `json:"source"`
	Line                int              `json:"line"`
	Consumes            string           `json:"consumes,omitempty"`
	Produces            string           `json:"produces,omitempty"`
	Queries             []queryParameter `json:"queryParameters,omitempty"`
	RequestBody         bool             `json:"requestBody,omitempty"`
	RequestBodyRequired bool             `json:"requestBodyRequired,omitempty"`
}

type queryParameter struct {
	Name     string  `json:"name"`
	Required bool    `json:"required"`
	Default  *string `json:"default,omitempty"`
}

var (
	classMappingPattern  = regexp.MustCompile(`@RequestMapping\s*\(\s*"([^"]*)"`)
	methodMappingPattern = regexp.MustCompile(`@(GetMapping|PostMapping|PutMapping|PatchMapping|DeleteMapping)(.*)`)
	stringPattern        = regexp.MustCompile(`"([^"]*)"`)
	pathParameterPattern = regexp.MustCompile(`\{([^}/]+)\}`)
	requestParamPattern  = regexp.MustCompile(`@RequestParam(?:\s*\(([^)]*)\))?\s+[\w<>?,.\[\]]+\s+(\w+)`)
	requestParamName     = regexp.MustCompile(`(?:name|value)\s*=\s*"([^"]+)"`)
	requestParamDefault  = regexp.MustCompile(`defaultValue\s*=\s*"([^"]*)"`)
	envPattern           = regexp.MustCompile(`\$\{([A-Z0-9_]+)(?::([^}]*))?\}`)
)

func Generate(repositoryRoot string) ([]GeneratedFile, error) {
	routes, err := inspectRoutes(repositoryRoot)
	if err != nil {
		return nil, err
	}
	clientTypes, aliases, err := inspectClientMessageTypes(repositoryRoot)
	if err != nil {
		return nil, err
	}
	serverTypes, err := inspectServerMessageTypes(repositoryRoot)
	if err != nil {
		return nil, err
	}
	environment, err := inspectEnvironment(repositoryRoot)
	if err != nil {
		return nil, err
	}

	assets := []struct {
		path  string
		value any
	}{
		{"contracts/http/routes.json", map[string]any{"version": 1, "routeCount": len(routes), "routes": routes}},
		{"contracts/http/openapi.yaml", openAPI(routes)},
		{"contracts/http/golden/scenarios.json", httpScenarios()},
		{"contracts/ws/client-messages.schema.json", messageSchema("MusicParty WebSocket client messages", clientTypes)},
		{"contracts/ws/server-messages.schema.json", messageSchema("MusicParty WebSocket server messages", serverTypes)},
		{"contracts/ws/scenarios/catalog.json", wsScenarios(clientTypes, aliases)},
		{"contracts/config/environment.yaml", map[string]any{"version": 1, "variables": environment}},
	}
	files := make([]GeneratedFile, 0, len(assets))
	for _, asset := range assets {
		content, err := json.MarshalIndent(asset.value, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", asset.path, err)
		}
		files = append(files, GeneratedFile{Path: asset.path, Content: append(content, '\n')})
	}
	return files, nil
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

func inspectRoutes(repositoryRoot string) ([]route, error) {
	controllers := filepath.Join(repositoryRoot, "src", "main", "java", "org", "thornex", "musicparty", "controller")
	entries, err := os.ReadDir(controllers)
	if err != nil {
		return nil, fmt.Errorf("read controllers: %w", err)
	}
	var routes []route
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".java") || entry.Name() == "ReactiveStreamUtils.java" || entry.Name() == "MusicSocketController.java" {
			continue
		}
		path := filepath.Join(controllers, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(string(content), "\n")
		base := ""
		for index, line := range lines {
			if match := classMappingPattern.FindStringSubmatch(line); len(match) == 2 {
				base = match[1]
			}
			mapping := methodMappingPattern.FindStringSubmatch(line)
			if len(mapping) == 0 {
				continue
			}
			annotation := line
			signature := ""
			for cursor := index; cursor < len(lines) && cursor <= index+12; cursor++ {
				if cursor > index {
					annotation += " " + strings.TrimSpace(lines[cursor])
				}
				signature += " " + strings.TrimSpace(lines[cursor])
				trimmed := strings.TrimSpace(lines[cursor])
				if !strings.HasPrefix(trimmed, "@") && (strings.HasSuffix(trimmed, "{") || strings.Contains(trimmed, ") {")) {
					break
				}
			}
			method := strings.ToUpper(strings.TrimSuffix(mapping[1], "Mapping"))
			localPath := ""
			if literal := stringPattern.FindStringSubmatch(annotation); len(literal) == 2 {
				localPath = literal[1]
			}
			fullPath := joinPath(base, localPath)
			operation := strings.TrimSuffix(entry.Name(), ".java") + ".unknown"
			if name := javaMethodName(signature); name != "" {
				operation = strings.TrimSuffix(entry.Name(), ".java") + "." + name
			}
			queries := []queryParameter{}
			for _, match := range requestParamPattern.FindAllStringSubmatch(signature, -1) {
				name := match[2]
				if configured := requestParamName.FindStringSubmatch(match[1]); len(configured) == 2 {
					name = configured[1]
				}
				parameter := queryParameter{Name: name, Required: !strings.Contains(match[1], "required = false") && !strings.Contains(match[1], "required=false")}
				if configured := requestParamDefault.FindStringSubmatch(match[1]); len(configured) == 2 {
					parameter.Default = &configured[1]
					parameter.Required = false
				}
				queries = append(queries, parameter)
			}
			sort.Slice(queries, func(i, j int) bool { return queries[i].Name < queries[j].Name })
			hasRequestBody := strings.Contains(signature, "@RequestBody") || strings.Contains(annotation, "MULTIPART_FORM_DATA")
			routes = append(routes, route{
				Method: method, Path: fullPath, Operation: operation,
				Controller: strings.TrimSuffix(entry.Name(), ".java"),
				Source:     filepath.ToSlash(filepath.Join("src/main/java/org/thornex/musicparty/controller", entry.Name())),
				Line:       index + 1, Queries: uniqueQueries(queries), RequestBody: hasRequestBody,
				RequestBodyRequired: hasRequestBody && !strings.Contains(signature, "@RequestBody(required = false)") && !strings.Contains(signature, "@RequestBody(required=false)"),
				Consumes:            mediaValue(annotation, "consumes"), Produces: mediaValue(annotation, "produces"),
			})
		}
	}
	routes = append(routes,
		route{Method: "GET", Path: "/actuator", Operation: "Actuator.discovery", Controller: "Actuator", Source: "src/main/resources/application.yml", Line: 4},
		route{Method: "GET", Path: "/actuator/health", Operation: "Actuator.health", Controller: "Actuator", Source: "src/main/resources/application.yml", Line: 15},
		route{Method: "GET", Path: "/actuator/health/liveness", Operation: "Actuator.liveness", Controller: "Actuator", Source: "src/main/resources/application.yml", Line: 15},
		route{Method: "GET", Path: "/actuator/health/readiness", Operation: "Actuator.readiness", Controller: "Actuator", Source: "src/main/resources/application.yml", Line: 15},
		route{Method: "GET", Path: "/actuator/info", Operation: "Actuator.info", Controller: "Actuator", Source: "src/main/resources/application.yml", Line: 12},
		route{Method: "GET", Path: "/actuator/metrics", Operation: "Actuator.metrics", Controller: "Actuator", Source: "src/main/resources/application.yml", Line: 12},
		route{Method: "GET", Path: "/actuator/metrics/{name}", Operation: "Actuator.metric", Controller: "Actuator", Source: "src/main/resources/application.yml", Line: 12},
		route{Method: "GET", Path: "/actuator/prometheus", Operation: "Actuator.prometheus", Controller: "Actuator", Source: "src/main/resources/application.yml", Line: 12, Produces: "text/plain"},
		route{Method: "GET", Path: "/", Operation: "Static.index", Controller: "Static", Source: "src/main/resources/static/index.html", Line: 1, Produces: "text/html"},
		route{Method: "GET", Path: "/{path}", Operation: "SpaRedirectController.fallback", Controller: "Static", Source: "src/main/java/org/thornex/musicparty/config/SpaRedirectController.java", Line: 19, Produces: "text/html"},
	)
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes, nil
}

func openAPI(routes []route) map[string]any {
	paths := map[string]any{}
	for _, item := range routes {
		parameters := []any{}
		for _, match := range pathParameterPattern.FindAllStringSubmatch(item.Path, -1) {
			parameters = append(parameters, map[string]any{"name": match[1], "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
		}
		for _, query := range item.Queries {
			schema := map[string]any{"type": "string"}
			if query.Default != nil {
				schema["default"] = *query.Default
			}
			parameters = append(parameters, map[string]any{"name": query.Name, "in": "query", "required": query.Required, "schema": schema})
		}
		operation := map[string]any{
			"operationId": strings.ReplaceAll(item.Operation, ".", "_"),
			"tags":        []string{item.Controller},
			"parameters":  parameters,
			"responses": map[string]any{
				"200": map[string]any{"description": "Java baseline success response"},
				"204": map[string]any{"description": "Java baseline success without a body"},
				"400": map[string]any{"$ref": "#/components/responses/BadRequest"},
				"401": map[string]any{"$ref": "#/components/responses/Unauthorized"},
				"403": map[string]any{"$ref": "#/components/responses/Forbidden"},
				"404": map[string]any{"$ref": "#/components/responses/NotFound"},
				"429": map[string]any{"$ref": "#/components/responses/TooManyRequests"},
				"500": map[string]any{"$ref": "#/components/responses/InternalError"},
			},
			"x-java-source": fmt.Sprintf("%s:%d", item.Source, item.Line),
		}
		if item.Operation == "SpaRedirectController.fallback" {
			operation["x-spring-catch-all"] = true
			operation["description"] = "SPA fallback for extensionless non-api, non-ws paths"
		}
		if item.RequestBody {
			contentType := "application/json"
			if item.Consumes != "" {
				contentType = item.Consumes
			}
			operation["requestBody"] = map[string]any{"required": item.RequestBodyRequired, "content": map[string]any{contentType: map[string]any{"schema": map[string]any{}}}}
		}
		pathItem, _ := paths[item.Path].(map[string]any)
		if pathItem == nil {
			pathItem = map[string]any{}
			paths[item.Path] = pathItem
		}
		pathItem[strings.ToLower(item.Method)] = operation
	}
	errorResponse := func(description string) map[string]any {
		return map[string]any{"description": description, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Error"}}}}
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "MusicParty frozen Java HTTP contract", "version": "go-rewrite-baseline-v1"},
		"servers": []any{map[string]any{"url": "http://localhost:8080"}},
		"paths":   paths,
		"components": map[string]any{
			"schemas": map[string]any{"Error": map[string]any{"type": "object", "properties": map[string]any{"message": map[string]any{"type": "string"}}, "additionalProperties": true}},
			"responses": map[string]any{
				"BadRequest": errorResponse("Bad request"), "Unauthorized": errorResponse("Authentication required"),
				"Forbidden": errorResponse("Permission denied or CSRF rejected"), "NotFound": errorResponse("Resource not found"),
				"TooManyRequests": errorResponse("Rate limit exceeded"), "InternalError": errorResponse("Unexpected Java baseline failure"),
			},
			"securitySchemes": map[string]any{"sessionCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "MP_SESSION"}, "csrfHeader": map[string]any{"type": "apiKey", "in": "header", "name": "X-CSRF-Token"}},
		},
		"x-contract-rules": map[string]any{"preserveStatus": true, "preserveMissingFields": true, "preserveArrayOrder": true, "preserveErrorType": true},
	}
}

func inspectClientMessageTypes(repositoryRoot string) ([]string, map[string]string, error) {
	path := filepath.Join(repositoryRoot, "src/main/java/org/thornex/musicparty/controller/MusicSocketController.java")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	text := string(content)
	start, end := strings.Index(text, "switch (type)"), strings.Index(text, "default ->")
	if start < 0 || end < start {
		return nil, nil, fmt.Errorf("cannot locate websocket dispatch switch")
	}
	values := stringPattern.FindAllStringSubmatch(text[start:end], -1)
	aliases := map[string]string{}
	canonical := []string{}
	var current string
	for _, match := range values {
		value := match[1]
		if strings.HasPrefix(value, "/") {
			if current != "" {
				aliases[value] = current
			}
			continue
		}
		if strings.Contains(value, ".") || strings.HasPrefix(value, "enqueue") {
			current = value
			canonical = append(canonical, value)
		}
	}
	return uniqueSorted(canonical), aliases, nil
}

func inspectServerMessageTypes(repositoryRoot string) ([]string, error) {
	root := filepath.Join(repositoryRoot, "src/main/java/org/thornex/musicparty")
	pattern := regexp.MustCompile(`(?:sendToSession|broadcastRoom|broadcastAll)\s*\([^;]*?"([a-zA-Z0-9_.-]+)"`)
	types := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".java") {
			return walkErr
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllSubmatch(content, -1) {
			types = append(types, string(match[1]))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return uniqueSorted(types), nil
}

func messageSchema(title string, types []string) map[string]any {
	variants := make([]any, 0, len(types))
	for _, messageType := range types {
		variants = append(variants, map[string]any{
			"type": "object", "required": []string{"type", "payload"}, "additionalProperties": false,
			"properties": map[string]any{
				"type": map[string]any{"const": messageType}, "requestId": map[string]any{"type": []string{"string", "null"}},
				"roomId": map[string]any{"type": []string{"string", "null"}}, "payload": map[string]any{},
			},
		})
	}
	return map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "title": title, "oneOf": variants, "x-message-types": types, "x-order-is-contractual": true}
}

func inspectEnvironment(repositoryRoot string) ([]map[string]any, error) {
	path := filepath.Join(repositoryRoot, "src/main/resources/application.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	variables := []map[string]any{}
	for lineIndex, line := range strings.Split(string(content), "\n") {
		for _, match := range envPattern.FindAllStringSubmatch(line, -1) {
			variables = append(variables, map[string]any{"name": match[1], "default": match[2], "source": fmt.Sprintf("src/main/resources/application.yml:%d", lineIndex+1)})
		}
	}
	sort.Slice(variables, func(i, j int) bool { return variables[i]["name"].(string) < variables[j]["name"].(string) })
	return variables, nil
}

func httpScenarios() map[string]any {
	return map[string]any{
		"version":       1,
		"normalization": map[string]any{"fields": []string{"publicId", "userId", "uuid-shaped id", "inv_-prefixed id", "lastLoginAt", "createdAt", "updatedAt", "expiresAt", "serverReceiveTime", "serverSendTime", "serverTimestamp", "timestamp", "joinedAt", "lastSeenAt", "usedAt", "revokedAt", "sessionToken"}, "headers": []string{"date"}, "cookies": []string{"MP_SESSION", "MP_CSRF", "MP_ADMIN_ELEVATION"}},
		"strict":        map[string]any{"status": true, "fieldPresence": true, "arrayOrder": true, "errorBody": true, "headerPresence": true},
		"scenarios": []any{
			map[string]any{"id": "health", "method": "GET", "path": "/actuator/health", "expectedStatus": 200},
			map[string]any{"id": "account-status", "method": "GET", "path": "/api/account/status", "expectedStatus": 200},
			map[string]any{"id": "account-me-unauthenticated", "method": "GET", "path": "/api/account/me", "expectedStatus": 401},
			map[string]any{"id": "registration-csrf-rejected", "method": "POST", "path": "/api/account/register", "body": map[string]any{}, "omitCsrf": true, "expectedStatus": 403},
			map[string]any{"id": "admin-login-invalid", "method": "POST", "path": "/api/account/login", "body": map[string]any{"username": "contract-admin", "password": "wrong-password"}, "expectedStatus": 401},
			map[string]any{"id": "admin-login", "action": "login", "method": "POST", "path": "/api/account/login", "expectedStatus": 200},
			map[string]any{"id": "registration-retired", "method": "POST", "path": "/api/account/register", "body": map[string]any{}, "expectedStatus": 410},
			map[string]any{"id": "account-me", "method": "GET", "path": "/api/account/me", "expectedStatus": 200},
			map[string]any{"id": "csrf-missing", "method": "PUT", "path": "/api/account/profile", "body": map[string]any{"displayName": "Contract Admin"}, "omitCsrf": true, "expectedStatus": 403},
			map[string]any{"id": "profile-update", "method": "PUT", "path": "/api/account/profile", "body": map[string]any{"displayName": "契约管理员"}, "expectedStatus": 200},
			map[string]any{"id": "rooms-list", "method": "GET", "path": "/api/rooms", "expectedStatus": 200},
			map[string]any{"id": "config", "method": "GET", "path": "/api/config", "expectedStatus": 200},
			map[string]any{"id": "platforms", "method": "GET", "path": "/api/platforms", "expectedStatus": 200},
			map[string]any{"id": "search-empty", "method": "GET", "path": "/api/search/local/contract", "query": map[string]string{"offset": "0", "limit": "20"}, "expectedStatus": 200},
			map[string]any{"id": "search-invalid-parameters", "method": "GET", "path": "/api/search/local/contract", "query": map[string]string{"offset": "-1", "limit": "20"}, "expectedStatus": 500},
			map[string]any{"id": "search-upstream-failure", "method": "GET", "path": "/api/search/netease/contract", "query": map[string]string{"offset": "0", "limit": "20"}, "expectedStatus": 502},
			map[string]any{"id": "invite-not-found", "method": "GET", "path": "/api/join/not-a-real-invite/metadata", "expectedStatus": 404},
			map[string]any{"id": "local-media-forbidden", "method": "GET", "path": "/api/local/media/not-a-real-track", "expectedStatus": 403},
			map[string]any{"id": "logout", "method": "POST", "path": "/api/account/logout", "expectedStatus": 204},
			map[string]any{"id": "account-me-after-logout", "method": "GET", "path": "/api/account/me", "expectedStatus": 401},
		},
	}
}

func wsScenarios(clientTypes []string, aliases map[string]string) map[string]any {
	return map[string]any{
		"version": 1, "endpoint": "/ws?room-id={roomId}", "cookie": "MP_SESSION", "originRequired": true,
		"aliases": aliases, "allClientMessageTypes": clientTypes,
		"strict": map[string]any{"messageOrder": true, "fieldPresence": true, "arrayOrder": true, "closeCode": true},
		"scenarios": []any{
			map[string]any{"id": "unauthorized-handshake", "expectCloseCode": 1008},
			map[string]any{"id": "resync", "send": []any{map[string]any{"type": "player.resync", "requestId": "resync-1", "roomId": "lounge", "payload": map[string]any{}}}, "expectTypes": []string{"player.state"}},
			map[string]any{"id": "ping", "send": []any{map[string]any{"type": "sync.ping", "requestId": "ping-1", "roomId": "lounge", "payload": map[string]any{"pingId": "contract-ping", "clientSendTime": 1234}}}, "expectTypes": []string{"sync.pong"}},
			map[string]any{"id": "identity-and-presence", "send": []any{map[string]any{"type": "user.me", "payload": map[string]any{}}, map[string]any{"type": "users.online", "payload": map[string]any{}}}, "expectTypes": []string{"user.me", "users.online"}},
			map[string]any{"id": "queue-version-gap", "send": []any{map[string]any{"type": "queue.reorder", "requestId": "reorder-gap", "roomId": "lounge", "payload": map[string]any{"oldIndex": -1, "newIndex": 0, "mutationId": "reorder-gap"}}}, "expectTypes": []string{"queue.reorder.nack"}},
			map[string]any{"id": "chat-message", "send": []any{map[string]any{"type": "chat.message", "payload": map[string]any{"content": "契约聊天消息 🎵"}}}, "expectTypes": []string{"chat.message"}},
			map[string]any{"id": "chat-history", "send": []any{map[string]any{"type": "chat.history.fetch", "payload": map[string]any{"limit": 20}}}, "expectTypes": []string{"chat.history"}},
			map[string]any{"id": "public-chat-message", "send": []any{map[string]any{"type": "public-chat.message", "payload": map[string]any{"content": "契约公共消息"}}}, "expectTypes": []string{"public-chat.message"}},
			map[string]any{"id": "public-chat-history", "send": []any{map[string]any{"type": "public-chat.history.fetch", "payload": map[string]any{"limit": 20}}}, "expectTypes": []string{"public-chat.history"}},
			map[string]any{"id": "reconnect-resync", "reconnect": true, "send": []any{map[string]any{"type": "player.resync", "requestId": "reconnect-resync", "roomId": "lounge", "payload": map[string]any{}}}, "expectTypes": []string{"player.state"}},
		},
	}
}

func joinPath(base, local string) string {
	path := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(local, "/")
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func javaMethodName(signature string) string {
	for _, visibility := range []string{" public ", " protected "} {
		start := strings.Index(" "+signature, visibility)
		if start < 0 {
			continue
		}
		remainder := (" " + signature)[start+len(visibility):]
		open := strings.IndexByte(remainder, '(')
		if open < 0 {
			continue
		}
		prefix := strings.TrimSpace(remainder[:open])
		fields := strings.Fields(prefix)
		if len(fields) > 0 {
			return fields[len(fields)-1]
		}
	}
	return ""
}

func mediaValue(annotation, key string) string {
	if !strings.Contains(annotation, key) {
		return ""
	}
	if strings.Contains(annotation, "MULTIPART_FORM_DATA") {
		return "multipart/form-data"
	}
	if strings.Contains(annotation, "TEXT_PLAIN") {
		return "text/plain"
	}
	if match := stringPattern.FindStringSubmatch(annotation); len(match) == 2 && strings.Contains(match[1], "/") {
		return match[1]
	}
	return ""
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		if _, ok := seen[value]; ok || value == "" {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueSorted(values []string) []string {
	values = unique(values)
	sort.Strings(values)
	return values
}

func uniqueQueries(values []queryParameter) []queryParameter {
	seen := map[string]struct{}{}
	result := []queryParameter{}
	for _, value := range values {
		if _, ok := seen[value.Name]; ok || value.Name == "" {
			continue
		}
		seen[value.Name] = struct{}{}
		result = append(result, value)
	}
	return result
}
