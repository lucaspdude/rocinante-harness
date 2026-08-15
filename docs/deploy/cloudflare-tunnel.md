# Cloudflare Tunnel deployment

Cloudflare Tunnel is the **simplest** way to expose roc-harness
on a public hostname without opening inbound ports or maintaining
a TLS certificate.

## One-time setup

```bash
# 1. Install cloudflared.
curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg \
  | sudo tee /usr/share/keyrings/cloudflare-main.gpg >/dev/null
echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared focal main' \
  | sudo tee /etc/apt/sources.list.d/cloudflared.list
sudo apt-get update && sudo apt-get install -y cloudflared

# 2. Authenticate.
cloudflared tunnel login

# 3. Create the tunnel.
cloudflared tunnel create roc-harness
# This prints the tunnel UUID and a credentials file under
# ~/.cloudflared/<UUID>.json. Copy to /etc/cloudflared/.
```

## Run the tunnel

`/etc/cloudflared/config.yml`:

```yaml
tunnel: <UUID>
credentials-file: /etc/cloudflared/<UUID>.json

ingress:
  - hostname: api.example.com
    service: http://127.0.0.1:30179
  - hostname: app.example.com
    service: http://127.0.0.1:30178
  - service: http_status:404
```

```bash
sudo systemctl enable --now cloudflared
```

## DNS

In the Cloudflare dashboard, create two CNAME records:

| Type  | Name | Target                 |
| ----- | ---- | ---------------------- |
| CNAME | api  | `<UUID>`.cfargotunnel.com |
| CNAME | app  | `<UUID>`.cfargotunnel.com |

Cloudflare issues the public TLS certificate automatically.

## Verifying

```bash
curl -sf https://api.example.com/api/v1/healthz
curl -sfL https://app.example.com/ | grep -q 'rocinante-harness'
```

## Operational notes

- The api sees requests as `127.0.0.1` (the cloudflared process),
  so the rate limiter sees one bucket per LAN client. Cloudflare
  adds `CF-Connecting-IP` — the api client.html can read this
  when present.
- Logs: `journalctl -u cloudflared` for the tunnel; the api log
  shows the proxied requests.
- Restart: `sudo systemctl restart cloudflared roc-harness`.
