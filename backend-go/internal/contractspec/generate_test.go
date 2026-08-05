package contractspec

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/httpapi"
	"github.com/stretchr/testify/require"
)

func TestGoRouteCatalogIsUniqueAndComplete(t *testing.T) {
	routes, err := httpapi.ContractRoutes()
	require.NoError(t, err)
	require.NotEmpty(t, routes)

	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		key := route.Method + " " + route.Path
		require.NotContains(t, seen, key)
		seen[key] = struct{}{}
		require.Equal(t, "backend-go/internal/httpapi", route.Source)
	}
	require.Contains(t, seen, "GET /api/rooms")
	require.Contains(t, seen, "GET /ws")
	require.Contains(t, seen, "GET /actuator/health/readiness")
}

func TestGeneratedContractsAreGoOnly(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	files, err := Generate(repositoryRoot)
	require.NoError(t, err)
	require.Len(t, files, 14)
	for _, file := range files {
		content := string(file.Content)
		require.NotContains(t, content, "src/main/java")
		require.NotContains(t, content, "x-java-source")
		require.NotContains(t, strings.ToLower(content), "frozen java")
	}
}

func TestEnvironmentContractComesFromGoConfig(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	variables, err := inspectGoEnvironment(repositoryRoot)
	require.NoError(t, err)
	require.NotEmpty(t, variables)

	byName := make(map[string]environmentVariable, len(variables))
	for _, variable := range variables {
		byName[variable.Name] = variable
	}
	require.Equal(t, "http://netease-api:3000", byName["NETEASE_API_URL"].Default)
	require.True(t, byName["NETEASE_COOKIE"].Secret)
	require.True(t, byName["SPRING_PROFILES_ACTIVE"].Deprecated)
}
