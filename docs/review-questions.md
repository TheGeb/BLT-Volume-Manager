# CI/CD Review — Open Questions

Before making changes, please clarify these items:

## 1. Nix-flake write access (Critical #1)

The `nix-flake` job in `ci.yml:136-161` has `contents: write` and pushes to the repo.
It runs on PRs, so a crafty PR could abuse this.

**Options:**
- Split nix hash updates into a separate workflow that fires **only on push to main** (not PRs). The PR workflow stays read-only.
- Keep one workflow but conditionally grant `contents: write` only on push events (via `github.event_name` check).

**Answer: Split the workflows.** Keep the existing PR CI read-only and move hash calculation, commit, and push into a separate workflow triggered only by pushes to `main` (with an optional manual dispatch). The updater should have job-level `contents: write`; the rest of CI should retain `contents: read`. This is a stronger boundary than a conditional because untrusted PR code never runs in a job that has write permission or a token intended to push changes.

## 2. Make lint targets should be read-only (High #11)

`lint`/`lint-go` call `format` (golangci-lint fmt), and `lint` calls `npm run lint:fix` — all mutating.
`dev-driver`, `dev-web` also call `ui-dev-build` which runs `lint:fix`.

**Options:**
- Split into read-only `lint` (golangci-lint run, staticcheck) and a separate `fix` target (format + lint:fix). Update dev targets accordingly.
- Keep Makefile as-is for developer convenience; just ensure CI uses explicit read-only commands.

**Answer: Split the targets.** `lint`, `lint-go`, and the CI commands should only inspect files. In the current Makefile, the UI portion already calls read-only `npm run lint`; the mutation comes from the shared `golangci-lint fmt` dependency and from `ui-dev-build`'s `npm run lint:fix`. Add an explicit `fix` target for `golangci-lint fmt` and the UI fixers. Make `ui-dev-build` check and build without fixing files, and let developers opt into `make fix` when they want formatting applied. This prevents a check from changing the worktree and makes local behavior match CI.

## 3. AWS SDK Renovate block rule — not found

The review says "current working tree adds a rule disabling github.com/aws/aws-sdk-go-v2 updates in renovate.json" — but `git diff HEAD` shows no changes to `renovate.json`, and the file has no such rule.

**Answer: It is not present in this checkout.** `renovate.json` contains no AWS SDK block rule, and the working-tree diff does not add one. Treat that review statement as hypothetical advice, not as an instruction to find a hidden file or branch. Do not add a rule disabling `github.com/aws/aws-sdk-go-v2` updates unless a separate, documented compatibility requirement is provided.

## 4. Nix hash update script (Medium #18)

`Makefile:43-81` has `nix-vendor-hash` and `nix-npm-hash` targets that modify `flake.nix` in-place and grep build output.

**Options:**
- Extract into a standalone script (`scripts/update-nix-hashes.sh`) with better cleanup/error handling.
- Keep in Makefile but add cleanup trap and better validation.

**Answer: Extract a standalone script.** Use `scripts/update-nix-hashes.sh` with strict error handling, a temporary copy or temporary working file, a trap that always restores/removes temporary state, validation that exactly one expected hash was extracted, and a nonzero exit on failed builds or malformed output. Keep small Make targets as wrappers if convenient. This makes the mutation logic testable and avoids duplicating fragile shell blocks in the Makefile.

## 5. Docker cache scopes (High #14)

`docker-bake.hcl` uses `type=gha` with no `scope=` for both `plugin` and `web` targets, so they share a cache and can conflict.

**Options:**
- Add `scope=plugin` and `scope=web` to the bake HCL cache lines.
- Switch CI to use `docker/build-push-action` directly with explicit cache scopes.

**Answer: Add explicit scopes in `docker-bake.hcl`.** Change the cache entries to `type=gha,scope=plugin` and `type=gha,scope=web` for both `cache-from` and `cache-to`. This is the smallest change, preserves the existing bake/release flow, and prevents the two targets from overwriting or evicting each other's cache records. There is no need to replace bake with separate build actions.

## 6. Renovate grouping (Renovate item #2)

All non-major deps are in one big group. Should nix/flake.lock updates be in their own group, separate from the "all non-major" group?

**Answer: Yes.** Put Nix/`flake.lock` updates in a dedicated non-major group, and exclude them from the general `all non-major dependencies` rule. Nix inputs can invalidate substantial build outputs and require different validation, so isolating them keeps unrelated application and tooling updates reviewable and allows the Nix group to be merged or deferred independently.

## 7. Release rehearsal (Low #33)

Add a non-publishing workflow that validates version normalization, bake tags, binary names, and embedded UI output on PRs. Does this belong as a separate workflow file, or as a CI job conditional on changes to VERSION?

**Answer: Use a separate workflow file.** Add a non-publishing `release-rehearsal` workflow triggered for pull requests that change `VERSION` (and the release/build inputs it validates), plus `workflow_dispatch` for manual checks. It should have read-only permissions, normalize an optional leading `v`, run the same UI build and Docker bake metadata checks used by release, build both `cmd/driver` and `cmd/web` for the expected architectures or inspect the generated artifacts, and verify that the UI is embedded. Keeping it separate makes the release-specific checks discoverable and avoids adding release logic and path-condition complexity to general CI.

## 8. `full` npm script (Medium #28)

`web/ui/package.json:17` ends with `go run .` which references no main package from the repo root. The actual entry points are `cmd/driver` and `cmd/web`. Should I fix this to a valid project command, or just remove the script entirely?

**Answer: Fix it, do not remove it.** The script is intended to provide an end-to-end local check, so retain it but invoke the real web entry point: `go run ./cmd/web --http-addr '0.0.0.0:8081'`. Remove the unsupported `--http-only` flag as well; `cmd/web` does not define that flag. The command will still require the normal runtime configuration and intentionally remains a long-running server command after the build and static checks complete.
