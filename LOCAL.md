# Local fork notes

- Upstream: `lox/notion-cli`
- Fork: `0xble/notion-cli`
- Maintained branch: `main`
- Install path: `~/.local/bin/notion-cli`
- Install entrypoint: `bin/upgrade`
- Runtime verification: `bin/smoke`

Current fork intent:

- keep upstream behavior intact where possible
- carry the post-reset fork changes on `main`
- version the installed runtime from the `v0.5.0` upstream base with a fork suffix
