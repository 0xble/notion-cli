# Profile authentication

Read on every maintenance run after the [root contract](../MAINTENANCE.md).
Apply its shared invariants and update rules before evaluating these records.
Preserve the behavior below during reconciliation and require its focused proof
before changing or retiring the patch. The root's overall verification also applies.

## NOTION-001: `feat(auth): add multi-profile support`

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
