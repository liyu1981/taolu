package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	libfossil "github.com/danmestas/go-libfossil"
	"gopkg.in/yaml.v3"

	"github.com/yli/taolu/pkg/commands"
	"github.com/yli/taolu/pkg/vault"
)

// Status is the response for GET /api/status.
type Status struct {
	ServerName      string                        `json:"server_name"`
	ServerVersion   string                        `json:"server_version"`
	VaultPath       string                        `json:"vault_path"`
	ProjectCode     string                        `json:"project_code"`
	TaoluCount      int                           `json:"taolu_count"`
	ArchivedCount   int                           `json:"archived_count"`
	Groups          []string                      `json:"groups"`
	Domains         []string                      `json:"domains"`
	UserDomain      string                        `json:"user_domain"`
	Authoring       string                        `json:"authoring"`
	Uptime          string                        `json:"uptime"`
	Installed       map[string]commands.InstalledInfo `json:"installed"`
	ForkUpstream    string                        `json:"fork_upstream,omitempty"`
	ForkSourceCommit string                       `json:"fork_source_commit,omitempty"`
}

// Config is the editable vault configuration exposed by the web UI.
type Config struct {
	UserDomain string `json:"user_domain"`
}

type configBody struct {
	UserDomain string `json:"user_domain"`
}

// handleConfig reads or updates the user's default taolu domain.
func handleConfig(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		switch r.Method {
		case http.MethodGet:
			domain, err := vault.GetUserDomain(repo)
			if err != nil {
				apiError(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, Config{UserDomain: domain})
		case http.MethodPut:
			var body configBody
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				apiError(w, http.StatusBadRequest, "invalid JSON body")
				return
			}
			if err := vault.SetUserDomain(repo, body.UserDomain); err != nil {
				apiError(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonOK(w, Config{UserDomain: body.UserDomain})
		default:
			apiError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}
}

// TaoluItem is a listing row for GET /api/taolus.
type TaoluItem struct {
	Name          string   `json:"name"`
	Group         string   `json:"group"`
	Domain        string   `json:"domain"`
	Mode          string   `json:"mode"`
	Description   string   `json:"description"`
	Tags          []string `json:"tags"`
	LatestVersion string   `json:"latest_version"`
	Archived      bool     `json:"archived"`
	ForkSource    string   `json:"fork_source,omitempty"`
}

// AssetMeta describes a files/ asset without its content.
type AssetMeta struct {
	Path string `json:"path"`
}

// TaoluDetail is the response for GET /api/taolus/{name}.
type TaoluDetail struct {
	Name       string      `json:"name"`
	Group      string      `json:"group"`
	Domain     string      `json:"domain"`
	Mode       string      `json:"mode"`
	Archived   bool        `json:"archived"`
	Skill      string      `json:"skill"`
	Action     string      `json:"action"`
	Assets     []AssetMeta `json:"assets"`
	LatestVer  string      `json:"latest_version"`
	VersionCnt int         `json:"version_count"`
	ForkSource string      `json:"fork_source,omitempty"`
}

// ContentFile is one file's content at a version (GET /api/taolus/{name}/content).
type ContentFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ContentResponse is the response for GET /api/taolus/{name}/content.
type ContentResponse struct {
	Name      string        `json:"name"`
	Version   string        `json:"version"`
	Archived  bool          `json:"archived"`
	Files     []ContentFile `json:"files"`
	AssetCnt  int           `json:"asset_count"`
}

// Version is one entry in GET /api/taolus/{name}/history.
type Version struct {
	Label   string    `json:"label"`
	UUID    string    `json:"uuid"`
	Date    time.Time `json:"date"`
	User    string    `json:"user"`
	Message string    `json:"message"`
}

// DiffFile is one file's unified diff between two versions.
type DiffFile struct {
	Path    string `json:"path"`
	Unified string `json:"unified"`
}

// DiffResponse is the response for GET /api/taolus/{name}/diff.
type DiffResponse struct {
	Name    string     `json:"name"`
	VersionA string    `json:"version_a"`
	VersionB string    `json:"version_b"`
	Files   []DiffFile `json:"files"`
}

// handleStatus reports server identity and vault summary.
func handleStatus(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, p, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		projectCode, err := repo.Config("project-code")
		if err != nil {
			apiError(w, http.StatusInternalServerError, "read project-code: "+err.Error())
			return
		}
		taolus, err := vault.ListTaolu(repo)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "list taolus: "+err.Error())
			return
		}
		archived, err := vault.ListArchivedTaolu(repo)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "list archived: "+err.Error())
			return
		}
		userDomain, _ := vault.GetUserDomain(repo)

		st := Status{
			ServerName:    serverName,
			ServerVersion: serverVersion(),
			VaultPath:     p,
			ProjectCode:   projectCode,
			TaoluCount:    len(taolus),
			ArchivedCount: len(archived),
			Groups:        sortedGroups(taolus),
			Domains:       sortedDomains(taolus),
			UserDomain:    userDomain,
			Uptime:        time.Since(startTime).Round(time.Second).String(),
			Installed:     commands.CheckInstalledGlobal(),
		}
		if upstream, commit, err := vault.VaultForkInfo(repo); err == nil {
			st.ForkUpstream = upstream
			st.ForkSourceCommit = commit
		}
		for _, t := range taolus {
			if t.Name == vault.SeedName {
				st.Authoring = t.LatestVersion
				break
			}
		}
		jsonOK(w, st)
	}
}

// handleTaolus lists taolus with optional filters.
func handleTaolus(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(r.URL.Query().Get("query"))
		group := r.URL.Query().Get("group")
		domain := r.URL.Query().Get("domain")
		include := r.URL.Query().Get("include")
		tag := r.URL.Query().Get("tag")
		showArchived := r.URL.Query().Get("archived") == "true"

		repo, _, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		var wanted map[string]bool
		if include != "" {
			wanted = map[string]bool{}
			for _, m := range strings.Split(include, ",") {
				wanted[strings.TrimSpace(m)] = true
			}
		}

		var items []TaoluItem
		collect := func(archived bool) error {
			var list []vault.TaoluInfo
			var err error
			if archived {
				list, err = vault.ListArchivedTaolu(repo)
			} else {
				list, err = vault.ListTaolu(repo)
			}
			if err != nil {
				return err
			}
			for _, t := range list {
				if archived != showArchived {
					continue
				}
				if group != "" && t.Group != group {
					continue
				}
				if domain != "" && t.Domain != domain {
					continue
				}
				if len(wanted) > 0 && !wanted[t.Mode] {
					continue
				}
				if tag != "" {
					if !hasTag(t.Tags, tag) {
						continue
					}
				}
				if q != "" {
					hay := strings.ToLower(t.Name + " " + t.Description + " " + t.Tags)
					if !strings.Contains(hay, q) {
						continue
					}
				}
			items = append(items, TaoluItem{
				Name:          t.Name,
				Group:         t.Group,
				Domain:        t.Domain,
				Mode:          t.Mode,
				Description:   t.Description,
				Tags:          splitTags(t.Tags),
				LatestVersion: t.LatestVersion,
				Archived:      archived,
				ForkSource:    t.ForkSource,
			})
			}
			return nil
		}

		if err := collect(false); err != nil {
			apiError(w, http.StatusInternalServerError, "list taolus: "+err.Error())
			return
		}
		if err := collect(true); err != nil {
			apiError(w, http.StatusInternalServerError, "list archived: "+err.Error())
			return
		}

		sort.Slice(items, func(i, j int) bool {
			if items[i].Domain == items[j].Domain {
				if items[i].Group == items[j].Group {
					return items[i].Name < items[j].Name
				}
				return items[i].Group < items[j].Group
			}
			return items[i].Domain < items[j].Domain
		})

		if items == nil {
			items = []TaoluItem{}
		}
		jsonOK(w, items)
	}
}

// handleTaolu returns the latest bundle of one taolu (skill/action/assets meta).
func handleTaolu(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		repo, _, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		skill, action, assets, err := vault.ReadTaoluBundle(repo, name, "")
		if err != nil {
			apiError(w, http.StatusNotFound, err.Error())
			return
		}
		archived, err := archivedFor(repo, name)
		if err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}
		mode := modeFor(repo, name)
		hist, err := vault.SkillHistory(repo, mustSkillPath(repo, name))
		if err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}

		meta := make([]AssetMeta, 0, len(assets))
		for _, a := range assets {
			meta = append(meta, AssetMeta{Path: a.Path})
		}
		d := TaoluDetail{
			Name:      name,
			Group:     groupFor(repo, name),
			Domain:    domainFor(repo, name),
			Mode:      mode,
			Archived:  archived,
			Skill:     skill,
			Action:    action,
			Assets:    meta,
			VersionCnt: len(hist),
		}
		if f, err := vault.ReadForkInfo(repo, vault.TaoluRef{Domain: d.Domain, Group: d.Group, Name: name}); err == nil && f != nil {
			d.ForkSource = f.Source.String()
		}
		if len(hist) > 0 {
			d.LatestVer = hist[len(hist)-1].Label
		}
		jsonOK(w, d)
	}
}

// handleHistory lists the versions of a taolu.
func handleHistory(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		repo, _, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		sp, err := vault.FindSkillPath(repo, name)
		if err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sp == "" {
			apiError(w, http.StatusNotFound, fmt.Sprintf("taolu %q not found in vault", name))
			return
		}
		hist, err := vault.SkillHistory(repo, sp)
		if err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}
		versions := make([]Version, 0, len(hist))
		for _, v := range hist {
			versions = append(versions, Version{
				Label:   v.Label,
				UUID:    vault.ShortUUID(v.UUID),
				Date:    v.Date,
				User:    v.User,
				Message: v.Message,
			})
		}
		jsonOK(w, versions)
	}
}

// handleContent returns the raw files of a taolu at a version.
func handleContent(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		version := r.URL.Query().Get("version")
		repo, _, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		skill, action, assets, err := vault.ReadTaoluBundle(repo, name, version)
		if err != nil {
			apiError(w, http.StatusNotFound, err.Error())
			return
		}
		archived, err := archivedFor(repo, name)
		if err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if version == "" {
			version = "tip"
		}

		files := []ContentFile{
			{Path: "SKILL.md", Content: skill},
			{Path: "ACTION.md", Content: action},
		}
		for _, a := range assets {
			files = append(files, ContentFile{Path: "files/" + a.Path, Content: a.Content})
		}
		jsonOK(w, ContentResponse{
			Name:     name,
			Version:  version,
			Archived: archived,
			Files:    files,
			AssetCnt: len(assets),
		})
	}
}

// handleDiff returns the per-file unified diff between two versions.
func handleDiff(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		versionA := r.URL.Query().Get("a")
		versionB := r.URL.Query().Get("b")
		if versionB == "" {
			apiError(w, http.StatusBadRequest, "missing required query param b (target version)")
			return
		}
		repo, _, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		sp, err := vault.FindSkillPath(repo, name)
		if err != nil {
			apiError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sp == "" {
			apiError(w, http.StatusNotFound, fmt.Sprintf("taolu %q not found in vault", name))
			return
		}
		uuidA, uuidB, err := vault.ResolveDiffVersions(repo, sp, versionA, versionB)
		if err != nil {
			apiError(w, http.StatusBadRequest, err.Error())
			return
		}
		ridA, err := repo.ResolveVersion(uuidA)
		if err != nil {
			apiError(w, http.StatusBadRequest, "resolve base version: "+err.Error())
			return
		}
		ridB, err := repo.ResolveVersion(uuidB)
		if err != nil {
			apiError(w, http.StatusBadRequest, "resolve target version: "+err.Error())
			return
		}

		action := strings.Replace(sp, "SKILL.md", "ACTION.md", 1)
		var files []DiffFile
		for _, f := range []string{sp, action} {
			entries, err := repo.Diff(ridA, ridB, f)
			if err != nil {
				apiError(w, http.StatusInternalServerError, "diff: "+err.Error())
				return
			}
			for _, e := range entries {
				files = append(files, DiffFile{Path: baseName(f), Unified: e.Unified})
			}
		}
		if files == nil {
			files = []DiffFile{}
		}
		jsonOK(w, DiffResponse{
			Name:     name,
			VersionA: versionAOrEmpty(uuidA),
			VersionB: versionB,
			Files:    files,
		})
	}
}

// --- helpers ---

func sortedGroups(taolus []vault.TaoluInfo) []string {
	g := vault.UniqueGroups(taolus)
	sort.Strings(g)
	return g
}

func sortedDomains(taolus []vault.TaoluInfo) []string {
	d := vault.UniqueDomains(taolus)
	sort.Strings(d)
	return d
}

func splitTags(tags string) []string {
	if strings.TrimSpace(tags) == "" {
		return []string{}
	}
	var out []string
	for _, t := range strings.Split(tags, ",") {
		if s := strings.TrimSpace(t); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func hasTag(tags, want string) bool {
	for _, t := range strings.Split(tags, ",") {
		if strings.TrimSpace(t) == want {
			return true
		}
	}
	return false
}

func archivedFor(r *libfossil.Repo, name string) (bool, error) {
	sp, err := vault.FindSkillPath(r, name)
	if err != nil {
		return false, err
	}
	if sp == "" {
		return false, nil
	}
	return vault.IsArchived(r, sp)
}

func mustSkillPath(r *libfossil.Repo, name string) string {
	sp, err := vault.FindSkillPath(r, name)
	if err != nil || sp == "" {
		return ""
	}
	return sp
}

func groupFor(r *libfossil.Repo, name string) string {
	sp := mustSkillPath(r, name)
	if sp == "" {
		return ""
	}
	ref, ok := vault.ParseTaoluPath(sp)
	if !ok {
		return ""
	}
	return ref.Group
}

func domainFor(r *libfossil.Repo, name string) string {
	sp := mustSkillPath(r, name)
	if sp == "" {
		return vault.DomainPrefix
	}
	ref, ok := vault.ParseTaoluPath(sp)
	if !ok {
		return vault.DomainPrefix
	}
	return ref.Domain
}

func modeFor(r *libfossil.Repo, name string) string {
	sp := mustSkillPath(r, name)
	if sp == "" {
		return ""
	}
	dir := strings.TrimSuffix(sp, "/SKILL.md")
	if data, err := r.ReadFileAt("tip", dir+"/ACTION.md"); err == nil {
		if mode := actionModeFrom(string(data)); mode != "" {
			return mode
		}
	}
	return vault.ModeInstall
}

// actionModeFrom extracts the action mode from an ACTION.md frontmatter.
func actionModeFrom(content string) string {
	rest := content
	if s := strings.TrimPrefix(content, "---\n"); s != content {
		rest = s
	} else if s := strings.TrimPrefix(content, "---\r\n"); s != content {
		rest = s
	} else {
		return ""
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	var m struct {
		Mode string `yaml:"mode"`
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), &m); err != nil {
		return ""
	}
	if m.Mode != "" && vault.ValidActionMode(m.Mode) {
		return m.Mode
	}
	return ""
}

func baseName(p string) string {
	parts := strings.Split(p, "/")
	return parts[len(parts)-1]
}

func versionAOrEmpty(uuidA string) string {
	if uuidA == "" {
		return ""
	}
	return uuidA
}

// mutationBody is the optional JSON body for mutation endpoints.
type mutationBody struct {
	Message string `json:"message"`
	User    string `json:"user"`
}

func decodeBody(r *http.Request) mutationBody {
	var b mutationBody
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&b)
	}
	return b
}

// handleArchive archives a taolu (commits an .archived marker).
func handleArchive(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		b := decodeBody(r)

		repo, _, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		if name == vault.SeedName {
			apiError(w, http.StatusBadRequest, "refusing to archive the built-in taolu-authoring guide")
			return
		}

		group, err := vault.ArchiveTaolu(repo, name, b.Message, b.User)
		if err != nil {
			apiError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonOK(w, map[string]string{
			"status": "archived",
			"group":  group,
			"name":   name,
		})
	}
}

// handleRestore restores an archived taolu (removes the .archived marker).
func handleRestore(vaultPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		b := decodeBody(r)

		repo, _, err := vault.OpenVault(vaultPath)
		if err != nil {
			apiError(w, http.StatusInternalServerError, "open vault: "+err.Error())
			return
		}
		defer repo.Close()

		group, err := vault.RestoreTaolu(repo, name, b.Message, b.User)
		if err != nil {
			apiError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonOK(w, map[string]string{
			"status": "restored",
			"group":  group,
			"name":   name,
		})
	}
}
