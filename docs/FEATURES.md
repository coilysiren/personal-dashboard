# Features

Inventory of what `personal-dashboard` ships today.

## Shape

Go daemon rendering a personal status dashboard from the catalog graph + repo-recall + other coilysiren/* signals. Designed for tailnet-only access on kai-server.

## Status

Bootstrap phase. Hello-world daemon + scaffold landed via #37; panels + infrastructure roll in via #38 through #48. See [#36 umbrella](https://github.com/coilysiren/personal-dashboard/issues/36).

## Inputs

- **catalog-graph** - reads `data/catalog-graph.yaml` from agentic-os-kai.
- **repo-recall** - tailnet endpoint on kai-server.
- **other coilysiren/* signals** - TBD per panel.

## Deploy

Tailnet-only. See [docs/deploy.md](deploy.md) for the rollout shape, [docs/dispatcher.md](dispatcher.md) for the dispatch surface.

## Dev loop

Direct `go` invocations are denied by the coily lockdown. Use `.coily/coily.yaml` verbs.

## See also

- [README.md](../README.md) - human-facing intro.
- [AGENTS.md](../AGENTS.md) - agent-facing operating rules (symlink to canonical).
- [.coily/coily.yaml](../.coily/coily.yaml) - allowlisted commands.

Cross-reference convention from [coilysiren/agentic-os#59](https://github.com/coilysiren/agentic-os/issues/59).
