# AGENTS.md

## Git commit rules

Write every commit in the project's house style. Never run `git commit`
yourself unless the user explicitly asks — produce the message and print it.

### 1. Title — conventional-commit style

Format: `<type>(<scope>): <imperative summary>`

- **type:** `feat`, `fix`, `refactor`, `style`, `chore`, `perf`, `docs`,
  `test`, `build`, `ci`.
- **scope:** the affected area, from this project's known scopes: `vault`,
  `practice`, `mcp`, `server`, `seed`, `cli`, `storage`, `docs`, `deps`,
  `config`, `tooling`. Multiple scopes are comma-separated, e.g.
  `chore(practice,seed): ...`.
- **summary:** concise and imperative ("wire", "add", "support", "render",
  "make"), not a full sentence. No trailing period.

### 2. Body — what and why, then the change list

After a blank line, write a detailed body:

1. **Context paragraph(s)** — what the code did before, the problem being
   solved, and why it mattered. Plain narrative ("Previously …"), not
   bullet-point-only prose.
2. **Solution paragraph** — the approach taken and how it resolves the issue.
3. **Bulleted change list** — one `- ` item per meaningful change, starting
   with the file/area touched, e.g. `- vault.go: skills are now versioned
   under practices/<group>/<name>/SKILL.md`. Group related lines under one
   bullet where natural.
4. **Verification line** — what was checked before finishing, e.g.
   `Verified: go build ./... && go vet ./...`.

### 3. Fix commits — root cause is mandatory

For `fix` commits the body **must** contain both:

- **The problem cause** — the root cause of the bug (what code path produced
  the wrong behavior and why).
- **The fix solution** — how the change resolves it.

### 4. House rules

- Use the imperative mood ("wire", "add", "fix", not "wires"/"added").
- One logical change per commit where practical.
- Merge commits from GitHub PRs are left as-is (`Merge pull request #N from
  <owner>/<branch>`).
- If a change is WIP or exploratory, mark it in the type/summary
  (`chore(...): ... WIP`) and say so in the body.
- Commit messages are not essays — the body should be thorough but each
  sentence must add information.
