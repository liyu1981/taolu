package vault

import (
	"fmt"

	libfossil "github.com/danmestas/go-libfossil"
)

// SaveTaolu commits skill and action content as a new version of the taolu.
// The pair is always changed together in a single check-in.
func SaveTaolu(r *libfossil.Repo, group, name, skill, action, message, user, versionLabel string) (label, uuid string, total int, err error) {
	if user == "" {
		user = "admin"
	}
	sp := skillPath(group, name)
	ap := actionPath(group, name)
	existing, err := FindSkillPath(r, name)
	if err != nil {
		return "", "", 0, err
	}
	if existing != "" && existing != sp {
		return "", "", 0, fmt.Errorf("taolu %q already exists under group %q (path %s); refusing to save under %q",
			name, skillGroup(existing), existing, group)
	}

	hist, err := SkillHistory(r, sp)
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
		message = fmt.Sprintf("save taolu %s (%s)", name, versionLabel)
	}

	rid, commitUUID, err := r.Commit(libfossil.CommitOpts{
		Files: []libfossil.FileToCommit{
			{Name: sp, Content: []byte(skill)},
			{Name: ap, Content: []byte(action)},
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
