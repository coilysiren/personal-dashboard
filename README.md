# personal-dashboard

Phone-first Pulse-style personal dashboard. Tailscale-only daemon on kai-server. Reads daily-* routine outputs, vault inbox, repo-recall, Sentry, Grafana, Phoenix, Bluesky, Discord, Reddit, Steam, Hue, Sonos, Cast. Writes through the coily web-ops MCP.

Inspired by [danielmiessler/Personal_AI_Infrastructure](https://github.com/danielmiessler/Personal_AI_Infrastructure)'s Pulse "Life Dashboard." Curly-Co-shaped: single daemon, plain-text substrate, no new DB, audit-clean writes through coily.

## Posture

- **Public repo, code-only.** No private data committed. Public-safe by construction.
- **Single daemon on kai-server.** Tailscale-only, systemd-managed.
- **Mobile-first PWA.** Phone is the design center. Desktop renders the same layout wider.
- **Plain-text substrate.** Reads existing files and runtime APIs. No new DB.
- **Redact-by-default.** Per-page reveal with per-session persistence. See [#41](https://github.com/coilysiren/personal-dashboard/issues/41).
- **Read-only at launch.** Every panel action routes through a dispatcher abstraction; the future coily web-ops MCP becomes the write backend. See [#40](https://github.com/coilysiren/personal-dashboard/issues/40).

## Data sources

Three git sources:

1. `coilysiren/personal-dashboard` (this repo) - code only.
2. `coilysiren/agentic-os-kai` - skills, `.coily/coily.yaml`, daily-* routine outputs.
3. `coilysiren/coilyco-vault` (private) - Obsidian inbox markdown.

Runtime sources (polled at request time, not committed): `~/.repo-recall/`, Sentry, Grafana / Tempo, Phoenix, Bluesky, Sirens Discord, Reddit, Steam, Hue, Sonos, Cast, ElevenLabs.

Local dashboard state at `~/.personal-dashboard/state/` on kai-server. Never written back to vault or agentic-os-kai.

## Layout

```
cmd/personal-dashboard/       daemon entrypoint
internal/server/              HTTP layer, embedded templates + static
internal/dispatcher/          single seam for panel writes (deep-link now, coily MCP later)
internal/panels/              one package per panel (#43 - #48)
internal/sources/             one package per data source (vault, sentry, grafana, etc.)
deploy/systemd/               unit file for kai-server install (#38)
.coily/coily.yaml             repo verbs allowed through coily lockdown
```

## Build and run locally

```
coily exec build        # compile
coily exec run          # start on 127.0.0.1:31337
coily exec test         # run tests
```

Direct `go` invocations are denied by the coily lockdown. Add verbs to `.coily/coily.yaml`.

## Status

Tracked under [#36 umbrella](https://github.com/coilysiren/personal-dashboard/issues/36). Bootstrap (#37) ships the scaffold and a hello-world daemon. Panels and infrastructure roll in via #38 through #48.

## See also

- [AGENTS.md](AGENTS.md) - agent-facing operating rules.
- [docs/FEATURES.md](docs/FEATURES.md) - inventory of what ships today.
- [.coily/coily.yaml](.coily/coily.yaml) - allowlisted commands.

Cross-reference convention from [coilysiren/agentic-os#59](https://github.com/coilysiren/agentic-os/issues/59).
