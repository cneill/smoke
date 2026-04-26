package agentsmd

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cneill/smoke/pkg/config"
)

const agentsFileName = "AGENTS.md"

// Discover scans for AGENTS.md files in two locations:
//  1. <config_dir>/AGENTS.md (user-level)
//  2. <projectPath>/AGENTS.md (project-level)
func Discover(projectPath string) (Catalog, error) {
	results := make(Catalog, 0, 2)

	// Check user-level file
	configDir, err := config.GetConfigDirPath()
	if err != nil {
		slog.Error("failed to search config dir for AGENTS.md files", "error", err)
	} else {
		homeFile := filepath.Join(configDir, agentsFileName)

		contents, err := os.ReadFile(homeFile)
		if err != nil {
			slog.Debug("no AGENTS.md found in home directory", "path", homeFile)
		} else {
			results = append(results, &File{
				Path:     homeFile,
				Contents: contents,
			})
		}
	}

	// Check project-level file
	projectFile := filepath.Join(projectPath, agentsFileName)

	contents, err := os.ReadFile(projectFile)
	if err != nil {
		slog.Debug("no AGENTS.md found in project directory", "path", projectFile)
	} else {
		results = append(results, &File{
			Path:     projectFile,
			Contents: contents,
		})
	}

	return results, nil
}
