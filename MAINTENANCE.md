# Maintenance

## Background

Maintained fork: `0xble/notion-cli` of `lox/notion-cli`; maintained and upstream-default
branch `main`. Canonical checkout: `/Users/brianle/notion-cli`. Accepted upstream
baseline: `aa6f492f0f70175ceb6620b213f4a5ee2c347024` (fetched 2026-09-09). Publish
only to `origin`; never push to `upstream`.

## Preserve

- Profile-scoped OAuth/API configuration remains portable, serialized, and reports
  refreshable authentication honestly.
- Official API search, local-image upload, page properties/icons, and data-source
  reads retain their tests; source sync, publication, installation, and runtime
  activation are separate stages requiring separate authorization and proof.

## Active patches

### NOTION-001: `feat(auth): add multi-profile support`

- **Status:** Active; source difference confirmed against `upstream/main` on 2026-09-09.
- **Provenance:** `bec90a0e0dbc4f7b2e0116cc7c9683e44cc78d08` plus follow-ups through
  `eb14477bbb525e06a936e1320a8e84b9cd2c8ec2`.
- **Surfaces/invariant:** `cmd/auth*.go`, `internal/config/`, and `internal/mcp/
  {oauth.go,token_store.go,token_lock_*.go}` keep profile names portable, refresh
  serialized, and status/list output truthful.
- **Proof:** `go test ./cmd ./internal/config ./internal/mcp`; **rollback:** revert
  the relevant auth family only after profile/config regression proof.
- **Upstream PR:** associated `https://github.com/lox/notion-cli/pull/35`, MERGED
  2026-06-20 per the 2026-09-09 audit; live-check released descendant before retirement.
- **Retire when:** released upstream passes the complete focused proof without fork delta.

### NOTION-002: `feat(page): upload standalone local images via api`

- **Status:** Active; source difference confirmed against `upstream/main` on 2026-09-09.
- **Provenance:** `0cf8e0a3f6a3dd9e02c59eb04812bf11a193bc4f` through
  `1b51ee07ff6acdddbe23f8843ebeae9d1a0cc171`.
- **Surfaces/invariant:** `cmd/page.go`, `internal/api/client.go`, and their tests
  preserve safe official-API image upload and request override behavior.
- **Proof:** `go test ./cmd ./internal/api`; **rollback:** revert this family after
  equivalent upload/error-path coverage. **Upstream PR:** associated
  `https://github.com/lox/notion-cli/pull/29`, MERGED 2026-04-30 per audit; live-check.
- **Retire when:** released upstream proves the complete behavior without the delta.

### NOTION-003: `feat(notion): add data source read commands`

- **Status:** Active; source difference confirmed against `upstream/main` on 2026-09-09.
- **Provenance:** `4af0de681956345d96cb74b425736a61c086537a` and
  `a8e86e8046cbae6de63b22128347fa282d8ff934`; **surfaces/invariant:**
  `cmd/{source.go,page_property.go,page_icon_test.go}` keep read/property/icon paths tested.
- **Proof:** `go test ./cmd ./internal/api`; **rollback:** revert both commits after
  command-interface equivalence. **Upstream issue/PR:** untracked; audit 2026-09-09
  records no association, not an absence claim. **Retire when:** released upstream
  passes the same proof with no fork-specific behavior.

## Update

Every run fetches `origin` and `upstream`, reconciles `main` onto latest
`upstream/main`, preserves only active recorded patches, live-checks each typed
upstream record, and runs `mise run test && mise run build` before authorized
publication. Update this contract with any patch addition/change/retirement;
missing or stale coverage blocks publication. Immediately before `Updated` or
`Already current`, fetch upstream again and require zero upstream-only commits;
otherwise report `Blocked` with failed stage, exact refs, and evidence. Publish
to `origin` or report that concrete blocker.

## Verify

```text
git diff --check
git rev-list --left-right --count upstream/main...main
```

Require a fresh final fetch with zero upstream-only commits and, after authorized
publication, local `main` SHA equal to `origin/main`. Installed/runtime SHA proof
is required only for separately authorized later stages.
