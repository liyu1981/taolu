# Plan: Extend Taolu Data Model to 3-Layer Domain/Category/Taolu

## Status: ✅ IMPLEMENTED

## Overview

Extend the current 2-layer `<category>/<taolu>` model to a 3-layer `<domain>/<category>/<taolu>` model, enabling namespaced taolu libraries across different sources.

## Implementation Summary

All four phases have been implemented and tested:

- **Phase 1**: Core data model with `TaoluRef` type and parsing functions
- **Phase 2**: Storage layer with domain-aware operations
- **Phase 3**: API layer with MCP tools, web API, and CLI updates
- **Phase 4**: Migration command and backward compatibility

## Key Changes

### New Files
- `pkg/vault/config.go` - Domain configuration helpers
- `pkg/vault/migrate.go` - Migration logic for 2-layer to 3-layer
- `cmd/taolu/migrate.go` - CLI migration command

### Modified Files
- `pkg/vault/practice.go` - Added `TaoluRef` type and parsing functions
- `pkg/vault/store.go` - Updated path resolution and listing functions
- `pkg/vault/save.go` - Added `SaveTaoluWithDomain` function
- `pkg/vault/mutate.go` - Updated rename function for 3-layer paths
- `pkg/tools/tools.go` - Updated MCP tools with domain support
- `pkg/web/handlers.go` - Updated web API with domain support
- `cmd/taolu/main.go` - Added migrate command

## Test Results

All tests pass:
```
ok  	github.com/yli/taolu/pkg/vault	6.751s
```

## Web UI Changes

Updated the web UI to merge Name and Group columns into a single "ID" column:

- **BrowseView.tsx**: 
  - Added domain filter dropdown
  - Merged Name and Group columns into single "ID" column showing `<domain>/<group>/<name>`
  - Updated table key to use full path

- **TaoluDetailView.tsx**:
  - Updated header to show full path: `<domain>/<group>/<name>`
  - Removed redundant group display

- **types.ts**:
  - Added `domain` field to `TaoluItem` and `TaoluDetail`
  - Added `domains` and `user_domain` fields to `Status`

- **api.ts**:
  - Added domain parameter to `taolus` function

## Current State

**2-layer model:**
```
taolus/<group>/<name>/SKILL.md
taolus/<group>/<name>/ACTION.md
taolus/<group>/<name>/files/...
```

**Example:**
```
taolus/frontend/local-first-webapp/SKILL.md
taolus/meta/taolu-authoring/SKILL.md
```

## Target State

**3-layer model:**
```
taolus/<domain>/<group>/<name>/SKILL.md
taolus/<domain>/<group>/<name>/ACTION.md
taolus/<domain>/<group>/<name>/files/...
```

**Examples:**
```
taolus/@local/frontend/local-first-webapp/SKILL.md
taolus/@liyu1981/frontend/local-first-webapp/SKILL.md
taolus/@local/meta/taolu-authoring/SKILL.md
```

## Design Decisions

### 1. Domain Format
- Domains are prefixed with `@` (e.g., `@liyu1981`, `@local`)
- `@local` is the well-known domain for the local Fossil vault
- User's own domain can be configured in Fossil's config table

### 2. Domain Resolution Rules
1. **Explicit domain**: `@liyu1981/frontend/local-first-webapp` → exact match
2. **Omitted domain**: `/frontend/local-first-webapp` → resolves based on user config:
   - If user has configured domain `@liyu191` → resolves to `@liyu1981/frontend/local-first-webapp`
   - If no user domain configured → resolves to `@local/frontend/local-first-webapp`
3. **Explicit @local**: `@local/frontend/local-first-webapp` → exact match

### 3. Fossil Config Storage
```sql
-- User's default domain (e.g., "@liyu1981")
INSERT INTO config(name, value, mtime) VALUES('user-domain', '@liyu1981', julianday('now'));

-- Domain aliases (JSON map)
INSERT INTO config(name, value, mtime) VALUES('domain-aliases', '{"@me":"@liyu1981"}', julianday('now'));
```

## Implementation Phases

### Phase 1: Core Data Model
**Files to modify:**
- `pkg/vault/practice.go` - Add TaoluRef type and parsing functions
- `pkg/vault/store.go` - Update path resolution logic

**Changes:**
1. Add `TaoluRef` struct:
```go
type TaoluRef struct {
    Domain string // "@local", "@liyu1981", etc.
    Group  string // "frontend", "backend", etc.
    Name   string // "local-first-webapp", etc.
}
```

2. Add parsing functions:
```go
func ParseTaoluRef(ref string) (TaoluRef, error)
func ResolveTaoluRef(ref string, userDomain string) TaoluRef
func (r TaoluRef) Path() string
func ParseTaoluPath(path string) (TaoluRef, bool)
func (r TaoluRef) String() string
```

3. Update path parsing:
```go
// Current: parseSkillPath(p string) (group, name string, ok bool)
// New: parseSkillPath(p string) (domain, group, name string, ok bool)
```

### Phase 2: Storage Layer
**Files to modify:**
- `pkg/vault/store.go` - Update FindSkillPath, ListTaolu
- `pkg/vault/save.go` - Update SaveTaolu for 3-layer paths
- `pkg/vault/mutate.go` - Update RenameTaolu for domain changes
- `pkg/vault/config.go` (new) - Domain configuration helpers

**Changes:**
1. Update `FindSkillPath`:
```go
// Current: FindSkillPath(r, name) (string, error)
// New: FindSkillPath(r, ref TaoluRef) (string, error)
```

2. Update `ListTaolu`:
```go
// Current: ListTaolu(r) ([]TaoluInfo, error)
// New: ListTaolu(r, domainFilter string) ([]TaoluInfo, error)
```

3. Add config helpers:
```go
func GetUserDomain(r *libfossil.Repo) (string, error)
func SetUserDomain(r *libfossil.Repo, domain string) error
func GetDomainAliases(r *libfossil.Repo) (map[string]string, error)
func SetDomainAliases(r *libfossil.Repo, aliases map[string]string) error
```

### Phase 3: API Layer
**Files to modify:**
- `pkg/tools/tools.go` - Update MCP tools
- `pkg/web/handlers.go` - Update web API handlers
- `cmd/taolu/` - Update CLI commands

**Changes:**
1. MCP Tools:
   - All tools accepting `name` now accept full references
   - New tool: `taolu_config` for domain configuration
   - `taolu_list` adds `domain` filter parameter

2. Web API:
   - All endpoints accepting `name` accept full references
   - New endpoint: `GET /api/config`
   - New endpoint: `PUT /api/config`

3. CLI:
   - `taolu config set user-domain @liyu1981`
   - `taolu config get user-domain`
   - All commands accept full references

### Phase 4: Migration & Compatibility
**Files to modify:**
- `cmd/taolu/migrate.go` (new) - Migration command
- `pkg/vault/migrate.go` (new) - Migration logic

**Changes:**
1. Add migration command:
```bash
taolu migrate --domain @local
```

2. Migration process:
   - List all existing 2-layer taolus
   - For each taolu:
     - Create new path: `taolus/@local/<group>/<name>/SKILL.md`
     - Move all files
     - Update origin marker for history continuity
     - Archive old directory
   - Commit changes with migration message

3. Backward compatibility:
   - Existing 2-layer paths recognized as `@local` domain
   - New saves use 3-layer format
   - No migration required for read access

## Example Usage

### Setting User Domain
```bash
taolu config set user-domain @liyu1981
```

### Saving a Taolu
```bash
# Full reference (explicit domain)
taolu save @liyu1981/frontend/my-skill SKILL.md ACTION.md

# Short reference (uses user domain)
taolu save frontend/my-skill SKILL.md ACTION.md

# Local domain (explicit)
taolu save @local/frontend/my-skill SKILL.md ACTION.md
```

### Listing Taolus
```bash
# All domains
taolu list

# Specific domain
taolu list --domain @liyu1981

# Local domain only
taolu list --domain @local

# User's domain
taolu list --domain @me
```

### Applying a Taolu
```bash
# Full reference
taolu apply @liyu1981/frontend/my-skill

# Short reference (uses user domain)
taolu apply frontend/my-skill
```

## Timeline

- **Week 1**: Core data model (Phase 1)
- **Week 2**: Storage layer (Phase 2)
- **Week 3**: API layer (Phase 3)
- **Week 4**: Migration & compatibility (Phase 4)

## Risks & Mitigations

1. **Risk**: Breaking existing workflows
   **Mitigation**: Backward compatibility for 2-layer paths during transition

2. **Risk**: Complex domain resolution
   **Mitigation**: Clear documentation and simple resolution rules

3. **Risk**: Migration data loss
   **Mitigation**: Origin markers preserve history, dry-run option

4. **Risk**: Performance impact
   **Mitigation**: Domain filtering is optional, lazy loading

## Success Criteria

1. ✅ 3-layer paths work correctly
2. ✅ Domain resolution works as specified
3. ✅ Backward compatibility maintained
4. ✅ Migration preserves version history
5. ✅ All API endpoints support full references
6. ✅ User domain configuration works
7. ✅ Documentation updated