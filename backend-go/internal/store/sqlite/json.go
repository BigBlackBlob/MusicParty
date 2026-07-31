package sqlite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func marshalJavaJSON(value any) (string, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(output.String(), "\n"), nil
}

func unmarshalJSON(value string, target any) error {
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("decode persisted JSON: %w", err)
	}
	return nil
}

func stringSetForJSON(values map[string]struct{}) []string {
	if values == nil {
		return []string{}
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	// Jackson serializes a Set in its iteration order. Go maps do not have one,
	// so sort to keep the persisted representation deterministic.
	sortStrings(result)
	return result
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for position := index; position > 0 && values[position] < values[position-1]; position-- {
			values[position], values[position-1] = values[position-1], values[position]
		}
	}
}
