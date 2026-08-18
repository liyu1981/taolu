package vault

import (
	"fmt"
	"path/filepath"
	"strings"

	libfossil "github.com/danmestas/go-libfossil"
)

// SaveTaolu commits skill, action, and any files/ assets as a new version of
// the taolu. The set is always changed together in a single check-in, and the
// assets argument is authoritative: assets from the previous version that are
// not listed are dropped rather than carried forward.
func SaveTaolu(r *libfossil.Repo, group, name, skill, action string, assets []Asset, message, user, versionLabel string) (label, uuid string, total int, err error) {
	return SaveTaoluWithDomain(r, DomainPrefix, group, name, skill, action, assets, message, user, versionLabel)
}

// SaveTaoluWithDomain commits skill, action, and any files/ assets as a new version of
// the taolu in the specified domain. This is the preferred function for new code.
func SaveTaoluWithDomain(r *libfossil.Repo, domain, group, name, skill, action string, assets []Asset, message, user, versionLabel string) (label, uuid string, total int, err error) {
	if user == "" {
		user = "admin"
	}
	if err := ValidateAssets(assets); err != nil {
		return "", "", 0, err
	}
	ref := TaoluRef{Domain: domain, Group: group, Name: name}
	sp := ref.Path()
	ap := ref.ActionPath()
	existing, err := FindSkillPath(r, name)
	if err != nil {
		return "", "", 0, err
	}
	if existing != "" && existing != sp {
		existingRef, _ := ParseTaoluPath(existing)
		return "", "", 0, fmt.Errorf("taolu %q already exists under domain %q group %q (path %s); refusing to save under %q",
			name, existingRef.Domain, existingRef.Group, existing, ref.Domain)
	}

	hist, err := SkillHistory(r, sp)
	if err != nil {
		return "", "", 0, err
	}
	if versionLabel == "" {
		versionLabel = fmt.Sprintf("v%d", len(hist)+1)
	}

	// Rewrite the tree so the new version reflects exactly SKILL.md,
	// ACTION.md, and the listed assets. The taolu's previous content files are
	// dropped (a save replaces them), while markers (.archived, origin) and all
	// other taolus are carried forward.
	tree, err := tipTree(r)
	if err != nil {
		return "", "", 0, err
	}
	files := make([]libfossil.FileToCommit, 0, len(tree)+len(assets)+2)
	if existing != "" {
		prefix := filepath.Dir(existing) + string(filepath.Separator)
		for _, f := range tree {
			if strings.HasPrefix(f.Name, prefix) {
				rel := strings.TrimPrefix(f.Name, prefix)
				if rel == "SKILL.md" || rel == "ACTION.md" ||
					strings.HasPrefix(rel, taoluFilesDir+string(filepath.Separator)) {
					continue
				}
			}
			files = append(files, f)
		}
	} else {
		files = append(files, tree...)
	}
	files = append(files,
		libfossil.FileToCommit{Name: sp, Content: []byte(skill)},
		libfossil.FileToCommit{Name: ap, Content: []byte(action)},
	)
	for _, a := range assets {
		files = append(files, libfossil.FileToCommit{
			Name:    ref.AssetPath(a.Path),
			Content: []byte(a.Content),
		})
	}

	if message == "" {
		message = fmt.Sprintf("save taolu %s (%s)", name, versionLabel)
	}

	rid, commitUUID, err := commitFullTree(r, files, message, user)
	if err != nil {
		return "", "", 0, err
	}
	if _, err := r.Tag(libfossil.TagOpts{
		Name:     name + "-" + versionLabel,
		TargetID: rid,
		User:     user,
	}); err != nil {
		return "", "", 0, fmt.Errorf("tag version: %w", err)
	}
	return versionLabel, commitUUID, len(hist) + 1, nil
}
