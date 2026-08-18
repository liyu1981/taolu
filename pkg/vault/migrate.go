package vault

import (
	"fmt"
	"path/filepath"
	"strings"

	libfossil "github.com/danmestas/go-libfossil"
)

// MigrateToDomain migrates all existing 2-layer taolus to a 3-layer format
// under the specified domain. This is a one-time operation that preserves
// version history via origin markers.
func MigrateToDomain(r *libfossil.Repo, domain, user, message string) (int, error) {
	if !ValidDomain(domain) {
		return 0, fmt.Errorf("invalid domain %q: must start with @ and be a valid slug", domain)
	}

	// Find all 2-layer taolus
	rid, err := r.ResolveVersion("tip")
	if err != nil {
		return 0, err
	}
	files, err := r.ListFiles(rid)
	if err != nil {
		return 0, err
	}

	var legacyPaths []string
	for _, f := range files {
		if !strings.HasPrefix(f.Name, taoluRoot+string(filepath.Separator)) {
			continue
		}
		if filepath.Base(f.Name) != "SKILL.md" {
			continue
		}
		// Check if this is a legacy 2-layer path
		parts := strings.Split(f.Name, string(filepath.Separator))
		if len(parts) == 4 && !strings.HasPrefix(parts[1], "@") {
			// taolus/group/name/SKILL.md -> legacy 2-layer
			legacyPaths = append(legacyPaths, f.Name)
		}
	}

	if len(legacyPaths) == 0 {
		return 0, nil
	}

	// Read the entire tree
	tree, err := tipTree(r)
	if err != nil {
		return 0, err
	}

	// Process each legacy taolu
	var newFiles []libfossil.FileToCommit
	for _, f := range tree {
		parts := strings.Split(f.Name, string(filepath.Separator))
		if len(parts) == 4 && !strings.HasPrefix(parts[1], "@") && parts[0] == taoluRoot {
			// This is a file in a legacy 2-layer taolu
			group := parts[1]
			name := parts[2]
			rel := parts[3]

			// Skip if this is not a valid taolu file
			if rel != "SKILL.md" && rel != "ACTION.md" &&
				!strings.HasPrefix(rel, taoluFilesDir+string(filepath.Separator)) &&
				rel != archivedMarker && rel != originMarker {
				// Stray file in taolu directory - keep as-is in new location
			}

			// Create new path with domain
			newPath := filepath.Join(taoluRoot, domain, group, name, rel)

			// For SKILL.md, we don't modify content
			// For origin marker, update to point to old path
			if rel == originMarker {
				oldDir := filepath.Join(group, name)
				newFiles = append(newFiles, libfossil.FileToCommit{
					Name:    newPath,
					Content: []byte(oldDir),
				})
			} else {
				newFiles = append(newFiles, libfossil.FileToCommit{
					Name:    newPath,
					Content: f.Content,
					Perm:    f.Perm,
				})
			}
		} else {
			// Keep all other files as-is
			newFiles = append(newFiles, f)
		}
	}

	if user == "" {
		user = "admin"
	}
	if message == "" {
		message = fmt.Sprintf("migrate taolus to %s domain", domain)
	}

	// Commit the migrated tree
	if _, _, err := commitFullTree(r, newFiles, message, user); err != nil {
		return 0, fmt.Errorf("commit migration: %w", err)
	}

	return len(legacyPaths), nil
}

// ListLegacyTaolus returns all 2-layer taolus that need migration.
func ListLegacyTaolus(r *libfossil.Repo) ([]TaoluRef, error) {
	rid, err := r.ResolveVersion("tip")
	if err != nil {
		return nil, err
	}
	files, err := r.ListFiles(rid)
	if err != nil {
		return nil, err
	}

	var legacy []TaoluRef
	for _, f := range files {
		if !strings.HasPrefix(f.Name, taoluRoot+string(filepath.Separator)) {
			continue
		}
		if filepath.Base(f.Name) != "SKILL.md" {
			continue
		}
		parts := strings.Split(f.Name, string(filepath.Separator))
		if len(parts) == 4 && !strings.HasPrefix(parts[1], "@") {
			legacy = append(legacy, TaoluRef{
				Domain: DomainPrefix,
				Group:  parts[1],
				Name:   parts[2],
			})
		}
	}
	return legacy, nil
}
