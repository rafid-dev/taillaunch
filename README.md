# TailLaunch

Open private tailnet web apps in their own lightweight browser window.

TailLaunch opens the official Tailscale sign-in page in your normal browser when needed, then opens the app in a dedicated Chromium-family app window. Closing that window ends the session. By default, session data is temporary and removed when TailLaunch exits; saved identity and browser data are opt-in with `--persist`.

**One binary. No system VPN or TUN adapter. No admin/root access required by TailLaunch.**

TailLaunch is an independent community project and is not affiliated with Tailscale Inc.

## Quick start

```bash
taillaunch https://my-server.my-tailnet.ts.net
```

TailLaunch finds an installed Microsoft Edge, Google Chrome, Brave, or Chromium browser and opens the target in a minimal app window with its own browser profile. If sign-in is needed, Tailscale's official page opens in your normal system browser. Finish sign-in there and TailLaunch opens the app automatically. TailLaunch never asks for your Tailscale password or writes the short-lived sign-in link to its logs.

Each launch uses temporary private connection state and a temporary browser profile. Closing the app window ends TailLaunch and removes both. A URL with an `http` or `https` scheme and a host is required; pass the URL as the first argument or with `--url`.

TailLaunch checks for a supported browser before starting sign-in. If none is installed, install Edge, Chrome, Brave, or Chromium and try again. On a connection error, check your internet connection and that your Tailscale account can access the app.

Use `--persist` to reuse a saved node identity and browser profile across launches:

```bash
taillaunch --persist https://my-server.my-tailnet.ts.net
```

Later persistent launches reconnect without asking you to authorize again while that identity remains valid.

### Portable mode

```bash
taillaunch --persist --portable https://my-server.my-tailnet.ts.net
```

With `--persist`, this keeps both the Tailscale node state and the dedicated browser profile under `taillaunch-data/` next to the executable. `--portable` alone does not make session data persistent.

### Low-memory mode

For older or low-RAM PCs:

```bash
taillaunch --persist --portable --low-memory https://my-server.my-tailnet.ts.net
```

This asks Chromium to use fewer renderer processes and smaller caches. It intentionally does not disable TLS, site isolation globally, or certificate validation.

### Browser window option

```bash
taillaunch --app=false https://my-server.my-tailnet.ts.net
```

By default, the target opens in Chromium's minimal app window. Use `--app=false` only if you want a full browser window with its normal controls.

### Proxy-only mode

```bash
taillaunch --proxy-only
```

TailLaunch connects to the tailnet and prints the local proxy address without launching a browser. This is useful for scripts or other applications that support HTTP proxies.

### Headscale / alternate control server

```bash
taillaunch --control-url https://headscale.example.com https://nas
```

## How it works

TailLaunch keeps private app access scoped to the session and does not modify system networking.

```text
Dedicated Edge / Chrome / Chromium / Brave app window
                  |
                  | HTTP proxy (loopback only)
                  v
          127.0.0.1:<random>
                  |
        +---------+---------+
        | routing policy    |
        +---------+---------+
          |               |
   tailnet host       public host
          |               |
       tsnet           direct TCP
          |               |
       tailnet           Internet
```

HTTPS is **not intercepted**. For HTTPS, the browser establishes a normal CONNECT tunnel through TailLaunch and performs TLS end-to-end with the destination.

## What gets routed through Tailscale?

TailLaunch sends these destinations through `tsnet`:

- Tailscale IPv4 (`100.64.0.0/10`) and IPv6 (`fd7a:115c:a1e0::/48`) addresses
- the current tailnet's MagicDNS suffix
- known peer hostnames
- single-label MagicDNS-style names such as `nas`
- subnet routes learned from current tailnet peers
- `*.ts.net`

Everything else is dialed directly through the machine's normal network connection.

## Supported targets

Release builds are produced for:

- Windows: `amd64`, `arm64`
- Linux: `amd64`, `arm64`
- macOS: `amd64`, `arm64`

The embedded networking layer is pure userspace Go. Browser launching currently targets Chromium-family browsers because they support a per-process `--proxy-server` flag without changing OS proxy settings.

## Security model

- TailLaunch binds its proxy only to loopback (`127.0.0.1`).
- Your browser still validates HTTPS certificates normally; TailLaunch does not MITM TLS.
- The embedded node receives only the permissions granted to that Tailscale identity by your tailnet policy.
- With `--persist`, the state directory contains a persistent Tailscale node identity. Treat it like a credential. Do not commit or share it.
- Public Internet traffic is sent directly unless it matches TailLaunch's tailnet routing policy.
- Closing TailLaunch shuts down the embedded `tsnet` node and local proxy. Session-only state and profile data are removed; persistent data remains for the next launch when `--persist` is used.

## Building

TailLaunch currently tracks Tailscale `v1.102.5` (the current stable client line as of October 2026) and uses Go `1.26+`.

```bash
go build -trimpath -o taillaunch ./cmd/taillaunch
```

For local development without downloading/building Tailscale, the repository includes a stub backend used only with the `stub` build tag:

```bash
go test -tags stub ./...
```

## Project status

Early MVP. The core architecture is intentionally small:

- [x] embedded `tsnet` node
- [x] first-run Tailscale sign-in through the normal browser
- [x] opt-in persistent node state
- [x] loopback HTTP/HTTPS CONNECT proxy
- [x] split tailnet/direct routing
- [x] Chromium-family browser launcher
- [x] Windows/Linux/macOS + amd64/arm64 release matrix
- [x] optional low-memory browser mode
- [ ] Firefox profile support
- [ ] small native tray/status UI
- [ ] automatic self-update
- [ ] signed/notarized release binaries
- [ ] richer service picker / tailnet peer discovery UI

## Related work

TailLaunch is inspired by the same general idea explored by:

- Tailscale's experimental `aperture-plus` embedded userspace browser
- Tailscale's experimental `ts-browser-ext`
- `OpenMinis/tsproxy`
- `kljensen/tailgate`

TailLaunch's focus is different: **a portable, cross-platform, one-click browser session with split routing and no system VPN installation.**

## License

MIT. See [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
