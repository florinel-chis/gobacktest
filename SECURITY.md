# Security Policy

## Local scanning

Three tools are used for local security scanning. Install them with `make tools`,
then run all three in one pass with `make security`.

| Tool | What it checks | Install |
|------|---------------|---------|
| **govulncheck** | Reachability-based Go CVE scanner (stdlib + deps). Only flags vulnerabilities in code paths actually reached by the module. | `go install golang.org/x/vuln/cmd/govulncheck@latest` |
| **gosec** | Static analysis rules for common Go security mistakes (hardcoded credentials, unsafe file ops, weak crypto, etc.). | `go install github.com/securego/gosec/v2/cmd/gosec@latest` |
| **osv-scanner** | Dependency vulnerability scan against the OSV database, driven from `go.mod`. | `go install github.com/google/osv-scanner/cmd/osv-scanner@latest` |

```
make tools     # install / update the three scanners
make security  # run all three; exit non-zero on any finding
make check     # vet + test + security (the full local gate)
```

---

## Current status

All three scanners are expected to be clean on this codebase. gosec's by-design sites
carry `// #nosec <rule> -- reason` annotations; each is documented below (one table row
per site):

| Location | Rule | Justification |
|----------|------|----------------|
| `optimize.go:65` | G404 | `rand.New(rand.NewSource(seed))` — deterministic seeded sampling for reproducible optimization; not used in a security context |
| `csv.go:16` | G304 | `os.Open(path)` — path is a caller-supplied API argument; documented in the API contract |
| `report/report.go:59` | G304 | `os.Create(path)` — output path is a caller-supplied API argument; documented in the API contract |
| `report/report.go:71` | G203 | `template.JS(lwcJS)` — vendored Lightweight Charts JS that we control, embedded verbatim; not user input |
| `report/report.go:72` | G203 | `template.JS(jsonBytes)` — our own `json.Marshal`'d backtest data, not user HTML |
| `report/heatmap.go:63` | G304 | `os.Create(path)` — output path is a caller-supplied API argument; documented in the API contract |
| `report/heatmap.go:189` | G203 | `template.CSS(...)` — CSS built only from hardcoded N/A color constants; no user input |
| `report/heatmap.go:197` | G203 | `template.CSS(...)` — CSS built only from hardcoded ramp color constants; no user input |
| `report/heatmap_test.go:47` | G304 | `os.ReadFile(path)` — test-controlled temp path inside `t.TempDir()` |

**Vendored asset:** Lightweight Charts v5.2.0 (Apache-2.0) — no known CVEs at
time of writing. Re-check this table whenever the version is bumped.

---

## Triage policy

When a new finding appears, answer these questions before acting:

1. **Is it reachable?** `govulncheck` only reports call-graph-reachable
   vulnerabilities. A `go.sum` hit without a reachable path is informational,
   not actionable.

2. **Real vulnerability or static false-positive?** gosec flags patterns; many
   are false-positives for a given call site. Read the rule documentation and
   understand whether the specific usage is actually unsafe.

3. **Is the risky input caller-controlled?** File-path or template arguments
   supplied by callers are by design (this is a library). The risk is owned by
   the caller; document it in the API and annotate accordingly.

4. **Is it dependency-only?** Check whether the vulnerable symbol is
   transitively imported and actually called. Use `govulncheck -show all` for
   full detail.

---

## Remediation process

### CVE in stdlib or a dependency

1. Run `make security` to confirm the finding and note the advisory ID.
2. For a stdlib CVE: bump the `go` / `toolchain` line in `go.mod` to a patched
   release (`go mod tidy`, then verify with `go build ./...`).
3. For a dependency CVE: `go get <module>@<patched-version>` then `go mod tidy`.
4. Re-run `make security` — all three scanners must pass before committing.
5. Commit with `fix: bump <module> to patch <advisory-id>`.

### Static finding (gosec)

- **Fix the code** whenever a cleaner implementation is straightforward.
- **Annotate with `#nosec`** only when the usage is intentional and safe by
  design. The annotation must include the rule ID and a concise human-readable
  reason:
  ```go
  f, err := os.Open(path) // #nosec G304 -- caller-supplied API argument
  ```
- **Never blanket-disable a rule** (e.g., `-exclude G304`) — that would silence
  all future findings of the same class across the whole module.
- Add a row to the table in "Current status" above for every new annotation.

---

## Pre-release security cadence (no CI, by design)

Run this checklist before tagging a release (and periodically on the default branch):

1. **`make check`** — vet + full test suite + `make security` (the full local gate).
2. **`make security`** — govulncheck + gosec + osv-scanner. All three must be clean.
   If `govulncheck` flags a *reachable* stdlib CVE, check the fix version in the
   finding, then bump the `go` directive in `go.mod` (`go mod edit -go=<x.y.z>` +
   `go mod tidy`) and re-scan.
3. **Toolchain freshness** — periodically check for a newer Go patch release even
   when no CVE is open (`GOTOOLCHAIN=goX.Y.Z go version` fetches it to test).
4. **Vendored asset** — on any Lightweight Charts bump, re-verify the pinned version
   in `NOTICE` and that `report/assets/…` matches the intended upstream release.
5. **`make parity`** — confirm the fixture matrix still holds after any toolchain bump.

Optional: a local `git` pre-push hook running `make check` keeps the gate enforced
without CI:

```sh
# .git/hooks/pre-push  (chmod +x)
#!/bin/sh
exec make check
```

---

## Reporting a vulnerability

If you discover a security vulnerability in this project, please report it
privately rather than opening a public issue, by contacting the maintainer
through GitHub.

You will receive an acknowledgement within 7 days. Please allow reasonable time
for a patch before any public disclosure.
