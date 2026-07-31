package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/contractspec"
)

func TestGeneratedContractsMatchJavaSource(t *testing.T) {
	repositoryRoot, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	files, err := contractspec.Generate(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := contractspec.Check(repositoryRoot, files); err != nil {
		t.Fatal(err)
	}
}

func findRepositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "pom.xml")); err == nil {
			if _, err := os.Stat(filepath.Join(directory, "backend-go", "go.mod")); err == nil {
				return directory, nil
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", os.ErrNotExist
		}
		directory = parent
	}
}
