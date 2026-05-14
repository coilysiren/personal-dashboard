# Deploying personal-dashboard on kai-server

The daemon runs as a system-level systemd unit on kai-server and is exposed
onto Kai's tailnet via `tailscale serve`. Source-of-truth deploy lives in
`coilysiren/infrastructure`. This doc is the runbook.

## Topology

* Daemon binds `127.0.0.1:31337`. No public bind. No laptop-localhost bind.
* `tailscale serve` proxies it onto `https://kai-server.<tailnet>.ts.net:8443`.
* systemd unit, user `kai`, restart on failure, starts at boot.

## One-time install on kai-server

```bash
bash /home/kai/projects/coilysiren/infrastructure/scripts/install-personal-dashboard.sh
```

The script idempotently:

1. `brew tap coilysiren/tap`
2. `brew install` or `brew upgrade coilysiren/tap/personal-dashboard`
3. `sudo install -m 0644 systemd/personal-dashboard.service /etc/systemd/system/personal-dashboard.service`
4. `sudo systemctl daemon-reload`
5. `sudo systemctl enable personal-dashboard.service`
6. `sudo systemctl restart personal-dashboard.service`

Re-run after every tap release to upgrade.

## Secrets

The unit reads `/etc/personal-dashboard.env` if present. Populate it before
the first start (or leave empty for the prototype). Format:

```ini
ELEVENLABS_API_KEY=...
ELEVENLABS_VOICE_ID=...
COILY_AUDIT_DIR=/home/kai/.coily/audit
PERSONAL_DASHBOARD_VAULT_PATH=/home/kai/projects/coilysiren/coilyco-vault
PERSONAL_DASHBOARD_STATE_DIR=/home/kai/.local/state/personal-dashboard
STEAM_API_KEY=...
STEAM_USER_ID=...
GRAFANA_URL=...
PHOENIX_URL=...
VICTORIAMETRICS_URL=...
BLUESKY_HANDLE=...
REDDIT_INBOX_RSS=...
```

Stash long-lived secrets in SSM and rehydrate the env file from there. Do
not commit the file.

## Tailnet exposure

After the service is up, expose it via `tailscale serve`. Port 443 on
kai-server is already taken by repo-recall, so personal-dashboard rides
8443:

```bash
sudo tailscale serve --bg --https=8443 http://127.0.0.1:31337
tailscale serve status
```

Final URL: `https://kai-server.<tailnet>.ts.net:8443`.

## Verify

* From Mac or phone on the tailnet:
  `curl -sfI https://kai-server.<tailnet>.ts.net:8443/`
* From outside the tailnet (drop wifi, switch phone to LTE):
  the same curl must fail.
* Reboot kai-server, confirm the service comes back:
  `coily ops personal-dashboard status`

## Operations

All managed via `coily` from any tailnet client:

* `coily personal-dashboard status` - systemctl status.
* `coily personal-dashboard tail` - journalctl -fu.
* `coily personal-dashboard restart` - daemon-reload + restart.
* `coily personal-dashboard stop` / `start` - enable/disable plus transition.
