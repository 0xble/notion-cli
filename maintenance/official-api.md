# Official API behavior

Read on every maintenance run after the [root contract](../MAINTENANCE.md).
Apply its shared invariants and update rules before evaluating these records.
Preserve the behavior below during reconciliation and require its focused proof
before changing or retiring the patch. The root's overall verification also applies.

## NOTION-002: `feat(page): upload standalone local images via api`

- **Provenance:** `0cf8e0a3f6a3dd9e02c59eb04812bf11a193bc4f` through
  `1b51ee07ff6acdddbe23f8843ebeae9d1a0cc171`.
- **Surfaces/invariant:** `cmd/page.go`, `internal/api/client.go`, and their tests
  preserve safe official-API image upload and request override behavior.
- **Proof:** `go test ./cmd ./internal/api`; **rollback:** revert this family after
  equivalent upload/error-path coverage. **Upstream PR:** associated
  `https://github.com/lox/notion-cli/pull/29`, MERGED 2026-04-30 per audit; live-check.
- **Retire when:** released upstream proves the complete behavior without the delta.

## NOTION-003: `feat(notion): add data source read commands`

- **Provenance:** `4af0de681956345d96cb74b425736a61c086537a` and
  `a8e86e8046cbae6de63b22128347fa282d8ff934`; **surfaces/invariant:**
  `cmd/source.go` keeps data-source reads and views tested.
- **Proof:** `go test ./cmd ./internal/api`; **rollback:** revert both commits only
  after command-interface equivalence. **Upstream issue/PR:** untracked; audit
  2026-09-09 records no association, not an absence claim. **Retire when:** released
  upstream passes the same proof with no fork-specific behavior.

## NOTION-004: `feat(page): retain full property reads and icon support`

- **Provenance:** `0e50ef7efab1504fa6c4f30361a12a88ae8dcff2`,
  `8fd7c2fb0d820acaf3d1fe0fb8da5ae969203b65`, and
  `14330663a4e86baf31fbddc416868124f2348dae`.
- **Surfaces/invariant:** `cmd/{page.go,page_property.go}` and `internal/api/` keep
  full property reads, icon updates, and page-ID validation covered.
- **Proof:** `go test ./cmd ./internal/api`; **rollback:** revert this family only
  after equivalent property, icon, and invalid-URL coverage. **Associated fork PRs:**
  `https://github.com/0xble/notion-cli/pull/13` and
  `https://github.com/0xble/notion-cli/pull/15` are closed; they are
  provenance, not evidence that upstream has the behavior. **Retire when:** released
  upstream passes the complete proof without this delta.

## NOTION-005: `fix(notion): retain official API search semantics`

- **Provenance:** `e6a35ed18428e54199c31652b8e4ede1cd94b3b7`; **surfaces/invariant:**
  `cmd/{official_search.go,search.go,db.go,page.go}`, `internal/api/`, and
  `internal/mcp/` retain tested official-search behavior.
- **Proof:** `go test ./cmd ./internal/api ./internal/mcp`; **rollback:** revert the
  provenance commit only after equivalent search and MCP coverage. **Upstream issue/PR:**
  untracked; audit 2026-09-09 records no association, not an absence claim. **Retire
  when:** released upstream passes the same proof without fork-specific behavior.
