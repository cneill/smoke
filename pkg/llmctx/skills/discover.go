package skills

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

const agentsSkillsDir = ".agents/skills"

// Discover scans for skill definitions in two locations:
//  1. $HOME/.agents/skills (user-level)
//  2. <projectPath>/.agents/skills (project-level)
//
// Each location is scanned for immediate subdirectories containing a SKILL.md file. Skills are deduplicated by name,
// with project-level skills taking precedence over user-level skills. Invalid skill files are logged as warnings but
// do not cause the overall discovery to fail.
func Discover(projectPath string) Catalog {
	// Use a map keyed by skill name for deduplication. We scan home first, then project, so project overwrites.
	byName := map[string]*Skill{}
	order := []string{}

	// 1. User-level skills
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("failed to determine home directory for skill discovery", "error", err)
	} else {
		homeSkillsDir := filepath.Join(home, agentsSkillsDir)
		discovered := discoverInDir(homeSkillsDir)

		for _, skill := range discovered {
			if _, exists := byName[skill.Name]; !exists {
				order = append(order, skill.Name)
			}

			byName[skill.Name] = skill
		}
	}

	// 2. Project-level skills
	projectSkillsDir := filepath.Join(projectPath, agentsSkillsDir)
	discovered := discoverInDir(projectSkillsDir)

	for _, skill := range discovered {
		if _, exists := byName[skill.Name]; !exists {
			order = append(order, skill.Name)
		}

		byName[skill.Name] = skill
	}

	// Build the catalog in stable order.
	catalog := make(Catalog, 0, len(order))
	for _, name := range order {
		catalog = append(catalog, byName[name])
	}

	return catalog
}

// discoverInDir scans a single directory for subdirectories containing a SKILL.md file, parses each, and returns the
// successfully parsed skills. Errors are logged as warnings.
func discoverInDir(dir string) []*Skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slog.Debug("skills dir does not exist", "path", dir)
		} else {
			slog.Warn("failed to read skills directory", "path", dir, "error", err)
		}

		return nil
	}

	var results []*Skill

	for _, entry := range entries {
		entryType := entry.Type()
		entryPath := filepath.Join(dir, entry.Name())
		skillPath := filepath.Join(entryPath, skillFileName)

		switch {
		case entryType&fs.ModeSymlink != 0:
			linkDest, err := os.Readlink(entryPath)
			if err != nil {
				slog.Error("failed to read skill link", "path", entryPath, "error", err)
				continue
			}

			if !filepath.IsAbs(linkDest) {
				linkAbs, err := filepath.Abs(linkDest)
				if err != nil {
					slog.Error("non-absolute link path is invalid", "path", entryPath, "link_destination", linkDest, "error", err)
					continue
				}

				skillPath = filepath.Join(linkAbs, skillFileName)
			}

		case !entryType.IsDir():
			slog.Debug("skipping non-symlink/non-directory skill file", "path", entryPath)
			continue
		}

		skill, err := ParseSkillFile(skillPath)
		if err != nil {
			slog.Warn("failed to parse skill file, skipping", "path", skillPath, "error", err)
			continue
		}

		results = append(results, skill)
	}

	return results
}
