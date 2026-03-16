# CLAUDE.md

<purpose>
gobreaker detects breaking changes in Go public APIs using `golang.org/x/exp/apidiff`.
It compares two versions (git refs or filesystem paths) and reports incompatible/compatible changes.
</purpose>

<architecture>
- `gobreaker.go` — public library API: `CompareRefs`, `CompareFilesystems`, `DetectDefaultBranch`, `Report`
- `cmd/gobreaker/main.go` — CLI entry point, uses `jessevdk/go-flags`
- `pkg/breaking/` — core diff logic (`diff.go`) and text formatting (`report.go`)
- `internal/git/` — git worktree checkout and filesystem comparison via `go-git`
</architecture>

<commands>
```bash
go build -o gobreaker ./cmd/gobreaker   # build CLI
go mod tidy                              # sync deps
./gobreaker v1.0.0                       # compare tag vs working dir
./gobreaker v1.0.0 v2.0.0               # compare two refs
./gobreaker -p /old/pkg /new/pkg         # compare filesystem paths
```
</commands>

<conventions>
- CLI flags/args go in `cmd/gobreaker/main.go`
- Breaking-change detection logic goes in `pkg/breaking/`
- Git operations go in `internal/git/` (uses go-git, temp worktrees)
- Public API lives in root `gobreaker.go` — keep it minimal
- Internal packages are excluded from analysis by default (pass `-i` to include)
- Exit code 1 when breaking changes detected
</conventions>

<versioning>
- Main branch: `latest`
- Semantic versioning via `semtag` (auto-detects bump level from API diff)
- Vendored dependencies (`vendor/`)
</versioning>
