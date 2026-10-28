package contractspec

import "github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"

// Optional metadata follows the existing Go decoder defaults. Identity fields
// must be present and contain a non-whitespace character.
func musicInputSchema() map[string]any {
	text := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max, "pattern": `\S`}
	}
	return map[string]any{
		"type": "object", "required": []string{"id", "name", "platform"},
		"properties": map[string]any{
			"id": text(128), "name": text(256), "platform": text(64),
			"artists":  map[string]any{"type": []string{"array", "null"}, "maxItems": 8, "items": map[string]any{"type": []string{"string", "null"}, "maxLength": 128}},
			"duration": map[string]any{"type": []string{"integer", "null"}, "minimum": 0, "maximum": 86400000},
			"coverUrl": map[string]any{"type": []string{"string", "null"}, "maxLength": 512},
		},
	}
}

func annotateHTTPValidation(item route, operation map[string]any) {
	var parameters []any
	maxLimit := 0
	keyword, keywordIn := "", ""
	switch item.Path {
	case "/api/search/{platform}/{keyword}":
		maxLimit, keyword, keywordIn = platform.MaxSearchLimit, "keyword", "path"
	case "/api/playlist/songs/{platform}/{playlistId}":
		maxLimit = platform.MaxPlaylistLimit
	case "/api/album/search/{platform}":
		keyword, keywordIn = "keyword", "query"
	case "/api/user/search/{platform}/{keyword}":
		keyword, keywordIn = "keyword", "path"
	case "/api/desktop/v1/search/{platform}", "/api/desktop/v1/albums/{platform}":
		maxLimit, keyword, keywordIn = platform.MaxSearchLimit, "q", "query"
	}
	if maxLimit > 0 {
		parameters = append(parameters,
			map[string]any{"in": "query", "name": "offset", "schema": map[string]any{"type": "integer", "minimum": 0, "maximum": platform.MaxOffset, "default": 0}},
			map[string]any{"in": "query", "name": "limit", "schema": map[string]any{"type": "integer", "minimum": 1, "maximum": maxLimit, "default": 20}})
	}
	if keyword != "" {
		parameters = append(parameters, map[string]any{"in": keywordIn, "name": keyword, "required": true, "description": "Nonblank search keyword, at most 128 UTF-8 bytes", "schema": map[string]any{"type": "string", "minLength": 1, "maxLength": 128, "pattern": `\S`, "x-max-utf8-bytes": 128}})
	}
	if len(parameters) > 0 {
		operation["parameters"] = parameters
		operation["responses"].(map[string]any)["400"] = map[string]any{"$ref": "#/components/responses/Error"}
	}
	if item.Method != "POST" && item.Method != "PUT" {
		return
	}
	var schema map[string]any
	switch item.Path {
	case "/api/me/playlists/{playlistId}/tracks/batch":
		schema = map[string]any{"type": "object", "required": []string{"musics"}, "properties": map[string]any{"musics": map[string]any{"type": "array", "minItems": 1, "maxItems": 200, "items": map[string]any{"$ref": "#/components/schemas/MusicInput"}}}}
	case "/api/rooms/{roomId}/playlists/{playlistId}/tracks":
		schema = map[string]any{"type": "object", "required": []string{"music"}, "properties": map[string]any{"music": map[string]any{"$ref": "#/components/schemas/MusicInput"}}}
	case "/api/me/liked-songs/{platform}/{musicId}":
		schema = musicInputSchema()
		schema["required"] = []string{"name"}
		properties := schema["properties"].(map[string]any)
		for _, field := range []string{"id", "platform"} {
			properties[field] = map[string]any{"type": []string{"string", "null"}, "description": "Replaced by the corresponding path parameter"}
		}
	}
	if schema != nil {
		operation["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": schema}}}
		operation["responses"].(map[string]any)["400"] = map[string]any{"$ref": "#/components/responses/Error"}
	}
}
