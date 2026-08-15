package vault

import (
	"fmt"

	libfossil "github.com/danmestas/go-libfossil"
)

// SavePractice commits content as a new version of the skill.
func SavePractice(r *libfossil.Repo, group, name, content, message, user, versionLabel string) (label, uuid string, total int, err error) {
	if user == "" {
		user = "admin"
	}
	targetPath := practicePath(group, name)
	existing, err := FindSkillPath(r, name)
	if err != nil {
		return "", "", 0, err
	}
	if existing != "" && existing != targetPath {
		return "", "", 0, fmt.Errorf("skill %q already exists under practice %q (path %s); refusing to save under %q",
			name, skillGroup(existing), existing, group)
	}

	hist, err := SkillHistory(r, targetPath)
	if err != nil {
		return "", "", 0, err
	}
	if versionLabel == "" {
		versionLabel = fmt.Sprintf("v%d", len(hist)+1)
	}

	parent, err := resolveParentTip(r)
	if err != nil {
		return "", "", 0, err
	}
	if message == "" {
		message = fmt.Sprintf("save %s (%s)", name, versionLabel)
	}

	rid, commitUUID, err := r.Commit(libfossil.CommitOpts{
		Files: []libfossil.FileToCommit{
			{Name: targetPath, Content: []byte(content)},
		},
		Comment:  message,
		User:     user,
		ParentID: parent,
	})
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
