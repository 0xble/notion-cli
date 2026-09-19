# Maintenance

## Background

Maintained fork: `0xble/notion-cli` of `lox/notion-cli`; the maintained branch is
`main`. The named upstream branch means upstream's live default branch, resolved on
every run before fetching; it is not statically pinned to `main`. Canonical checkout:
`/Users/brianle/notion-cli`. Accepted upstream
baseline: `aa6f492f0f70175ceb6620b213f4a5ee2c347024` (fetched 2026-09-09). Publish
only to `origin`; never push to `upstream`.

## Preserve

- Profile-scoped OAuth/API configuration remains portable, serialized, and reports
  refreshable authentication honestly.
- Official API search, local-image upload, page properties/icons, and data-source
  reads retain their tests; source sync, publication, installation, and runtime
  activation are separate stages requiring separate authorization and proof.

## Active patches

All entries are active; source differences were confirmed against upstream's
then-current default branch on 2026-09-09.

Read every linked support file on every maintenance run. This root is the sole
enrolled contract; support files extend its shared Preserve, Update, and Verify
requirements with the complete patch records and focused proof.

| Patch | Required behavior and record | When |
| --- | --- | --- |
| NOTION-001 | [Portable profiles and serialized authentication](maintenance/authentication.md) | Every run |
| NOTION-002 | [Safe official-API local-image uploads](maintenance/official-api.md) | Every run |
| NOTION-003 | [Data-source reads and views](maintenance/official-api.md) | Every run |
| NOTION-004 | [Full property reads and icon support](maintenance/official-api.md) | Every run |
| NOTION-005 | [Official API search semantics](maintenance/official-api.md) | Every run |
| NOTION-006 | [Fork version and install flow](maintenance/distribution.md) | Every run |

## Update

Every run resolves upstream's live default branch before fetching it, then fetches
`origin` and `upstream` separately, reconciles `main` onto latest
`upstream/$UPSTREAM_DEFAULT`, preserves only active recorded patches, live-checks
each typed upstream record, and runs `mise run test && mise run build` before
authorized publication. Update this contract with any patch addition/change/retirement;
missing or stale coverage blocks publication. Immediately before `Updated` or
`Already current`, resolve and fetch upstream's live default branch again and require
zero upstream-only commits; otherwise report `Blocked` with failed stage, exact refs,
and evidence. Publish to `origin` or report that concrete blocker.

## Verify

Require a fresh final fetch with zero upstream-only commits and, after authorized
publication, local `main` SHA equal to `origin/main`. Installed/runtime SHA proof
is required only for separately authorized later stages.
