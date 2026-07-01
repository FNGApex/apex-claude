# Auto-update — spec

Contract for the Apex Claude auto-update mechanism. Design + rationale: `docs/design/auto-update.md`.

Scope: pure-Go (stdlib only) update pipeline in the `apex` binary — version embedding, cached
release check, session-start nudge, `apex update` self-update of binary + artifacts in lockstep,
checksum verification — plus the publish-side changes that feed it. No LLM anywhere in the path.

## Fixed contracts

- Repo slug: `FNGApex/apex-claude` (const in `internal/update`, must equal install.ps1's `$Repo`).
- Release assets per tag: `apex-claude-<os>-<arch>.zip` for each Makefile `RELEASE_TARGETS` entry,
  `SHA256SUMS`, `install.ps1`. Bundle layout: `apex(.exe)`, `commands/ax-*.md`, `agents/ax-*.md`,
  `skills/ax-*/…`, `output-styles/apex.md`.
- `SHA256SUMS` format: one line per zip, `<lower-hex-sha256>  <filename>` (two spaces, coreutils
  `sha256sum` compatible).
- Latest-tag discovery: `GET https://github.com/FNGApex/apex-claude/releases/latest` without
  following redirects; tag = last path segment of the `Location` header. Timeout 5s.
- Version format: `vMAJOR.MINOR.PATCH`; compare as int triple. Non-matching tags are ignored.
- Cache file: `<os.UserCacheDir()>/apex-claude/update-check.json`, schema
  `{"checked_at":"<RFC3339>","latest":"vX.Y.Z"}`. TTL 24h. Failed checks stamp `checked_at` too.
- Opt-out env var: `APEX_NO_UPDATE_CHECK` (non-empty disables the hook-side check + nudge only).
- No cryptographic signatures — checksums only (explicit punt, see design §5).

## Checkpoints

Each row is independently implementable and verifiable.

| # | Checkpoint | Contract | Verify |
|---|------------|----------|--------|
| 1 | Version package | New `internal/version/version.go`: `package version; const Version = "0.2.0"`. `cmd/apex/main.go` drops its local const and prints `version.Version`. `scripts/publish.ps1` regexes the const from the new path; when `-Version` is passed explicitly, it dies if the tag != `v` + const. | `apex version` prints `apex 0.2.0`; `go test ./...` green; `publish.ps1 -DryRun -Version v9.9.9` fails with a mismatch error; `publish.ps1 -DryRun` (no -Version) resolves v0.2.0. |
| 2 | Layout package extraction | `artifactRoot()`, `looksLikeArtifactRoot()`, `isLooseInstall()`, `apexHooksWired()` move from `internal/doctor` to new `internal/layout` (exported). Doctor imports layout; doctor output and exit codes unchanged. | Existing doctor tests pass unchanged (relocated); `apex doctor` output identical before/after on both dev and loose layouts. |
| 3 | Release check + cache | `internal/update`: `LatestTag()` does the no-redirect GET (5s timeout) and parses the tag; `Compare(a, b)` orders `vX.Y.Z` triples; cache read/write at the fixed path via `os.UserCacheDir()` (dir created on demand; a UserCacheDir error degrades to no-cache, never an error). Failed network check stamps `checked_at`, preserves prior `latest`. Unit tests use `httptest` for the redirect and `t.TempDir` via an injectable cache path. | `go test ./internal/update/...` green: 302 parse, malformed tag ignored, compare table, TTL expiry, failure stamping. |
| 4 | `apex update check` | New `cmd/apex/cmd_update.go` registers `update`. `apex update check` refreshes the cache and prints `apex v0.2.0 — latest v0.3.0 (update available)` or `apex v0.2.0 — up to date`. Exit 0 = up to date, 1 = update available, 2 = check failed. `--quiet` suppresses stdout (same exit codes) — this is the hook-spawned form. Runs in any layout (check is layout-agnostic). | Run against a stubbed base URL (env or test seam `APEX_UPDATE_BASE_URL` override): all three exit codes reproducible; cache file written with fresh `checked_at`. |
| 5 | Session-start nudge | `internal/hooks.SessionStart` gains, before existing nudges: (a) best-effort remove of `<root>/bin/apex.exe.old`; (b) cache read — if cached `latest` > `version.Version`, append nudge `Apex update available: v<cur> → <latest> — run 'apex update'`; (c) if cache missing/older than 24h, spawn `os.Executable()` `update check --quiet` detached (Start + Release, nil stdio) and do not wait. `APEX_NO_UPDATE_CHECK` non-empty skips (b) and (c). Hook still returns 0 always, performs zero network I/O itself, and stays sub-second. | Unit: seeded cache with newer version → nudge line present in JSON `additionalContext`; equal/older version → absent; env var set → absent and no spawn. Manual: `time apex hooks session-start` < 100ms with no cache and no network. |
| 6 | SHA256SUMS on publish | `scripts/publish.ps1` computes `Get-FileHash -Algorithm SHA256` for every bundle zip, writes `dist/SHA256SUMS` in the fixed format, and includes it in `gh release create`/`upload` asset lists. | `publish.ps1 -DryRun` produces `dist/SHA256SUMS` with 5 lines; on a Unix box `cd dist && sha256sum -c SHA256SUMS` passes. |
| 7 | `apex update` happy path (Unix semantics) | `apex update [--to vX.Y.Z]`: refuse unless `layout.isLooseInstall(root)` (message: `dev layout — use 'git pull && make install'`, exit 2). Resolve tag (latest or `--to`); if tag == current and no `--to`, print up-to-date, exit 0. Download zip for `runtime.GOOS-GOARCH` + `SHA256SUMS` from that tag; verify hash (mismatch/missing line → exit 1, nothing touched); extract via `archive/zip` (zip-slip guarded: reject entries escaping the temp dir) to a temp dir; copy artifacts into root with installer semantics (overwrite `commands/ax-*.md`, `agents/ax-*.md`, `output-styles/apex.md`; wholesale-replace each `skills/ax-*` dir); binary: write `<root>/bin/.apex.new`, chmod 0755, `os.Rename` over `apex`. Post-update: warn (not fail) if `layout.apexHooksWired(root)` is false; rewrite cache as up-to-date; print `updated v<old> → <new>`. Exit 0 success, 1 verify/download failure, 2 usage/layout. | Integration test against `httptest` server serving a crafted bundle + SHA256SUMS into a temp loose root: artifacts and binary replaced, exit 0; corrupted hash → exit 1 and root untouched; dev-layout root → exit 2. Manual: `apex update --to v0.2.0` against the real release on a scratch `CLAUDE_CONFIG_DIR`. |
| 8 | Windows binary swap | On `runtime.GOOS == "windows"`: rename `apex.exe → apex.exe.old`, write new `apex.exe` (from the verified extract), on write failure rename `.old` back (rollback, exit 1). `.old` cleanup is best-effort at the start of `apex update` and in checkpoint 5(a); removal failure is silent. | Windows test (or CI windows runner): run installed `apex.exe update --to <same-tag>` — succeeds while running, `apex.exe.old` exists after, next `apex hooks session-start` removes it. Simulated write-failure test asserts rollback restores the original exe. |
| 9 | install.ps1 checksum verify | `scripts/install.ps1` downloads `SHA256SUMS` from the same tag it resolves the bundle from; verifies the zip via `Get-FileHash` before extracting; mismatch → die. If `SHA256SUMS` is absent (pre-checksum releases), warn and proceed. | Install a tag that has SHA256SUMS → passes; tamper one byte of a local zip in a mocked run → dies; point at a pre-checksum tag → warns and completes. |
| 10 | Registry + docs sync | Root `CLAUDE.md` backbone registry line gains `update`; `apex help` lists `update` with summary (automatic via `register()`). `.claude/project/signals.md` build/test table row for the update check is added on the next signals refresh (not hand-edited here). | `apex help` shows `update`; `grep update CLAUDE.md` hits the backbone registry line; `apex validate` and `apex doctor` pass. |

Suggested build order: 1 → 2 → 3 → 4 → 6 → 7 → 8 → 5 → 9 → 10 (nudge last among Go work so it
can spawn a real, working `update check`).

## Non-goals

- No signature verification (checksums only — design §5).
- No auto-apply: updates are always user-initiated via `apex update`; the system only nudges.
- No settings.json mutation during update (hook paths are stable absolute paths).
- No prerelease channel; `releases/latest` semantics only, `--to` for pinning.
- No changes to `scripts/install.sh` (source-checkout installs update via `git pull && make install`).

## Change log

- 2026-07-01 — Initial spec (ax-plan): version package, redirect-sniff check with 24h cache,
  detached background refresh + session-start nudge, `apex update` lockstep self-update with
  SHA256SUMS verification and Windows rename dance, publish.ps1 checksum emission.
