package contractspec

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGoFrontendSchemasHaveConcretePayloads(t *testing.T) {
	for _, messages := range [][]messageSpec{clientMessageSpecs(), serverMessageSpecs()} {
		schema := websocketSchema("test", messages)
		variants, ok := schema["oneOf"].([]any)
		require.True(t, ok)
		require.Len(t, variants, len(messages))

		for _, rawVariant := range variants {
			variant := rawVariant.(map[string]any)
			properties := variant["properties"].(map[string]any)
			payload := properties["payload"].(map[string]any)
			require.NotEmpty(t, payload["type"])
			if payload["type"] == "object" {
				if _, typedReference := payload["x-typescript-type"]; !typedReference {
					require.Contains(t, payload, "properties")
					require.Equal(t, false, payload["additionalProperties"])
				}
			}
		}
	}
}

func TestGeneratedFrontendTypesAreStrictAndVersioned(t *testing.T) {
	files := goFrontendFiles()
	require.Len(t, files, 7)

	for _, file := range files {
		if strings.HasSuffix(file.Path, ".ts") {
			require.NotContains(t, string(file.Content), ": any")
		}
		if strings.HasSuffix(file.Path, ".json") {
			var value map[string]any
			require.NoError(t, json.Unmarshal(file.Content, &value))
			require.Equal(t, frontendContractVersion, value["version"])
		}
	}
}
