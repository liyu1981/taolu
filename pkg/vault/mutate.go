package vault

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	libfossil "github.com/danmestas/go-libfossil"
)

// tipTree returns every file tracked at tip with its expanded content, ready
// to be committed again. An empty slice is returned when the repository has no
// check-ins yet.
func tipTree(r *libfossil.Repo) ([]libfossil.FileToCommit, error) {
	rid, err := r.ResolveVersion("tip")
	if errors.Is(err, libfossil.ErrVersionNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files, err := r.ListFiles(rid)
	if err != nil {
		return nil, err
	}
	out := make([]libfossil.FileToCommit, 0, len(files))
	for _, f := range files {
		data, err := r.ReadFileAt("tip", f.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, libfossil.FileToCommit{Name: f.Name, Content: data, Perm: f.Perm})
	}
	return out, nil
}

// findTaolu locates a taolu by name, verifying it exists and is not the
// built-in guide.
func findTaolu(r *libfossil.Repo, name string) (skillPath, group string, err error) {
	sp, err := FindSkillPath(r, name)
	if err != nil {
		return "", "", err
	}
	if sp == "" {
		return "", "", fmt.Errorf("taolu %q not found in vault", name)
	}
	if name == SeedName {
		return "", "", fmt.Errorf("refusing to modify the built-in %q guide", SeedName)
	}
	return sp, skillGroup(sp), nil
}

// commitFullTree commits the given file set as the complete new tip manifest,
// omitting any file not listed (used to remove files such as the archived
// marker). Returns the new check-in RID.
func commitFullTree(r *libfossil.Repo, files []libfossil.FileToCommit, message, user string) (int64, error) {
	parent, err := resolveParentTip(r)
	if err != nil {
		return 0, err
	}
	if user == "" {
		user = "admin"
	}
	if message == "" {
		message = "modify taolu"
	}
	rid, _, err := r.Commit(libfossil.CommitOpts{
		Files:           files,
		Comment:         message,
		User:            user,
		ParentID:        parent,
		PartialManifest: true,
	})
	if err != nil {
		return 0, err
	}
	return rid, nil
}

// ArchiveTaolu marks a taolu as archived by committing an .archived marker
// file into its directory. The source tree is untouched, but archived taolus
// are hidden from normal listings and refused by consuming tools until
// restored.
func ArchiveTaolu(r *libfossil.Repo, name, message, user string) (group string, err error) {
	sp, group, err := findTaolu(r, name)
	if err != nil {
		return "", err
	}
	archived, err := IsArchived(r, sp)
	if err != nil {
		return "", err
	}
	if archived {
		return "", fmt.Errorf("taolu %q is already archived", name)
	}
	marker := filepath.Join(filepath.Dir(sp), archivedMarker)
	parent, err := resolveParentTip(r)
	if err != nil {
		return "", err
	}
	if user == "" {
		user = "admin"
	}
	if message == "" {
		message = fmt.Sprintf("archive taolu %s", name)
	}
	if _, _, err := r.Commit(libfossil.CommitOpts{
		Files:    []libfossil.FileToCommit{{Name: marker, Content: nil}},
		Comment:  message,
		User:     user,
		ParentID: parent,
	}); err != nil {
		return "", err
	}
	return group, nil
}

// RestoreTaolu removes the .archived marker, bringing an archived taolu back
// into normal listings and use.
func RestoreTaolu(r *libfossil.Repo, name, message, user string) (group string, err error) {
	sp, group, err := findTaolu(r, name)
	if err != nil {
		return "", err
	}
	archived, err := IsArchived(r, sp)
	if err != nil {
		return "", err
	}
	if !archived {
		return "", fmt.Errorf("taolu %q is not archived", name)
	}
	marker := filepath.Join(filepath.Dir(sp), archivedMarker)
	tree, err := tipTree(r)
	if err != nil {
		return "", err
	}
	remaining := make([]libfossil.FileToCommit, 0, len(tree))
	for _, f := range tree {
		if f.Name == marker {
			continue
		}
		remaining = append(remaining, f)
	}
	if user == "" {
		user = "admin"
	}
	if message == "" {
		message = fmt.Sprintf("restore taolu %s", name)
	}
	if _, err := commitFullTree(r, remaining, message, user); err != nil {
		return "", err
	}
	return group, nil
}

// RenameTaolu moves a taolu to a new name, optionally into another group. It
// rewrites the SKILL.md frontmatter name, moves SKILL.md, ACTION.md, support
// files, and the archived marker to the new path, and records an origin marker
// so version history continues under the new name instead of restarting at v1.
func RenameTaolu(r *libfossil.Repo, name, newName, newGroup, message, user string) (oldGroup, g string, err error) {
	sp, oldGroup, err := findTaolu(r, name)
	if err != nil {
		return "", "", err
	}
	if !ValidSlug(newName) {
		return "", "", fmt.Errorf("invalid new name %q: must be 1-64 lowercase alphanumeric with single hyphen separators", newName)
	}
	if newGroup == "" {
		newGroup = oldGroup
	} else if !ValidSlug(newGroup) {
		return "", "", fmt.Errorf("invalid group %q: must be 1-64 lowercase alphanumeric with single hyphens", newGroup)
	}
	if existing, err := FindSkillPath(r, newName); err != nil {
		return "", "", err
	} else if existing != "" {
		return "", "", fmt.Errorf("taolu %q already exists under group %q; refusing to rename onto it", newName, skillGroup(existing))
	}

	skill, action, err := ReadTaoluAtVersion(r, name, "")
	if err != nil {
		return "", "", err
	}
	renamed, err := renameSkillFrontmatter(skill, newName)
	if err != nil {
		return "", "", err
	}

	tree, err := tipTree(r)
	if err != nil {
		return "", "", err
	}
	oldPrefix := filepath.Join(taoluRoot, oldGroup, name) + string(filepath.Separator)
	remaining := make([]libfossil.FileToCommit, 0, len(tree)+3)
	for _, f := range tree {
		if !strings.HasPrefix(f.Name, oldPrefix) {
			remaining = append(remaining, f)
			continue
		}
		switch filepath.Base(f.Name) {
		case "SKILL.md", "ACTION.md", originMarker:
			// Re-added below with the renamed frontmatter / new origin.
			continue
		}
		remaining = append(remaining, libfossil.FileToCommit{
			Name:    filepath.Join(taoluRoot, newGroup, newName, filepath.Base(f.Name)),
			Content: f.Content,
			Perm:    f.Perm,
		})
	}
	remaining = append(remaining,
		libfossil.FileToCommit{Name: skillPath(newGroup, newName), Content: []byte(renamed)},
		libfossil.FileToCommit{Name: actionPath(newGroup, newName), Content: []byte(action)},
		libfossil.FileToCommit{Name: filepath.Join(taoluRoot, newGroup, newName, originMarker),
			Content: []byte(filepath.Join(oldGroup, name))},
	)

	hist, err := SkillHistory(r, sp)
	if err != nil {
		return "", "", err
	}
	nextLabel := fmt.Sprintf("v%d", len(hist)+1)

	if user == "" {
		user = "admin"
	}
	if message == "" {
		message = fmt.Sprintf("rename taolu %s to %s", name, newName)
	}
	rid, err := commitFullTree(r, remaining, message, user)
	if err != nil {
		return "", "", err
	}
	if _, err := r.Tag(libfossil.TagOpts{
		Name:     newName + "-" + nextLabel,
		TargetID: rid,
		User:     user,
	}); err != nil {
		return "", "", fmt.Errorf("tag version: %w", err)
	}
	return oldGroup, newGroup, nil
}

// renameSkillFrontmatter rewrites the frontmatter name of a SKILL.md without
// reformatting any other line.
func renameSkillFrontmatter(content, newName string) (string, error) {
	raw, body, err := splitRawFrontmatter(content)
	if err != nil {
		return "", err
	}
	lines := strings.Split(raw, "\n")
	replaced := false
	for i, line := range lines {
		if replaced {
			break
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "name:") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "name: " + newName
		replaced = true
	}
	if !replaced {
		return "", errors.New("frontmatter has no name field")
	}
	return "---\n" + strings.Join(lines, "\n") + "\n---\n" + body, nil
}
