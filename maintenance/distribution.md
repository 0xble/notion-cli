# Fork build and distribution

Read on every maintenance run after the [root contract](../MAINTENANCE.md).
Apply its shared invariants and update rules before evaluating these records.
Preserve the behavior below during reconciliation and require its focused proof
before changing or retiring the patch. The root's overall verification also applies.

## NOTION-006: `fix(build): retain fork version/install flow`

- **Provenance:** `ad2a7a8fcf9001317dbbb95e3c57f6324819ea0a`; **surfaces/invariant:**
  `bin/{smoke,upgrade,version}`, `mise.toml`, `LOCAL.md`, and `README.md` preserve
  fork version and install guidance without performing installation.
- **Proof:** `sh -n bin/smoke bin/upgrade bin/version && mise run build`; **rollback:**
  revert the provenance commit only after equivalent build/version coverage. **Upstream
  issue/PR:** untracked; audit 2026-09-09 records no association, not an absence claim.
  **Retire when:** a separately authorized runtime migration removes this fork flow.
