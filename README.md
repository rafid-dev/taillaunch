# TailLaunch

TailLaunch is a lightweight, cross-platform tailnet browser launcher. It
connects to Tailscale in userspace and opens a private web app in its own
Chromium-family app window, without installing a system VPN, TUN adapter, or
requiring administrator/root access.

TailLaunch is an independent community project and is not affiliated with
Tailscale Inc.

## GUI quick start

The normal desktop launcher is `taillaunch-gui`:

```bash
taillaunch-gui
```

1. Click **Connect to Tailscale**.
2. Complete authentication on the official Tailscale page in your system
   browser. TailLaunch never asks for your Tailscale password.
3. After connecting, enter a tailnet hostname or full `http://`/`https://`
   address and click **Open App**.

GUI sessions are temporary by default: closing the private app removes the
session's Tailscale state and browser profile. Enable **Remember me on this
device** when you want the launcher to reuse saved identity and browser data
between launches. GUI memory mode defaults to **Auto**; **Normal** and **Low
memory** are available in **Settings**, along with advanced browser, control
server/Headscale, and logging options.

Private apps open maximized in a normal Chromium-family app window with its
ordinary title bar and controls. Microsoft Edge, Google Chrome, Brave, or
Chromium must be installed on the machine.

## Screenshots

The launcher starts disconnected, connects through the system browser, and
keeps advanced options in Settings.

![Disconnected launcher](docs/screenshots/launcher-disconnected.png)

![Connected launcher](docs/screenshots/launcher-connected.png)

![Settings](docs/screenshots/settings.png)

## CLI quick start

`taillaunch` is the command-line frontend, suitable for scripts and users who
prefer a terminal:

```bash
taillaunch https://my-server.my-tailnet.ts.net
```

The CLI also uses temporary connection state by default. Use `--persist` to
reuse a saved Tailscale node identity and browser profile:

```bash
taillaunch --persist https://my-server.my-tailnet.ts.net
```

Use `--app=false` for a full browser window, or `--low-memory` for explicit
conservative Chromium process and cache limits. `--portable` places persistent
data next to the executable, but only has an effect with `--persist`.

## Proxy-only mode

To connect to the tailnet without opening a browser, print a loopback proxy
address for another application:

```bash
taillaunch --proxy-only
```

The proxy listens only on `127.0.0.1`. HTTPS is not intercepted: browsers use
a normal CONNECT tunnel and perform TLS end-to-end with the destination.

## Headscale and alternate control servers

The CLI accepts an alternate control server or Headscale URL:

```bash
taillaunch --control-url https://headscale.example.com https://nas
```

The same option is available in the GUI's advanced Settings screen.

## How routing works

TailLaunch sends tailnet destinations through the embedded userspace Tailscale
node and sends other destinations through the machine's normal network
connection:

```text
Chromium-family app window
          |
          | loopback HTTP proxy
          v
  127.0.0.1:<random>
          |
    split routing policy
      |             |
   tailnet        public host
      |             |
    tsnet       direct TCP
```

Tailnet routing includes Tailscale IPv4 (`100.64.0.0/10`) and IPv6
(`fd7a:115c:a1e0::/48`) addresses, the current MagicDNS suffix, known peer
hostnames, subnet routes learned from peers, single-label MagicDNS-style names,
and `*.ts.net`.

## Supported targets and release artifacts

Release builds produce both `taillaunch` (CLI) and `taillaunch-gui` (GUI)
artifacts for:

- Windows: `amd64`, `arm64`
- Linux: `amd64`, `arm64`
- macOS: `amd64`, `arm64`

Each release also publishes `SHA256SUMS.txt`. The browser launcher currently
targets Chromium-family browsers because they support a per-process proxy
without changing the operating system's proxy settings.

## Security and data

- The local proxy binds only to loopback.
- HTTPS certificates are validated by the browser; TailLaunch does not MITM
  TLS.
- The embedded node receives only the permissions granted to that Tailscale
  identity by the tailnet policy.
- Persistent state contains a Tailscale node identity and should be treated
  like a credential. Do not commit or share it.
- Closing TailLaunch shuts down the embedded node and proxy. Temporary session
  data is removed; persistent data remains only when persistence was enabled.

## Building

```bash
go build -trimpath -o taillaunch ./cmd/taillaunch
go build -trimpath -o taillaunch-gui ./cmd/taillaunch-gui
```

For local development without downloading/building Tailscale, use the stub
backend:

```bash
go test -tags stub ./...
```

## Project status

The v0.2.0 release is GUI-focused and includes the shared `internal/app` core,
separate CLI and GUI frontends, launcher-style connection flow, opt-in
persistent sessions, Auto/Normal/Low memory modes, maximized app windows, and
advanced settings kept out of the main flow.

Current follow-up work includes Firefox profile support, native tray/status
integration, automatic self-update, signed/notarized binaries, and richer
service discovery.

## Related work

TailLaunch is inspired by the same general idea explored by:

- Tailscale's experimental `aperture-plus` embedded userspace browser
- Tailscale's experimental `ts-browser-ext`
- `OpenMinis/tsproxy`
- `kljensen/tailgate`

TailLaunch's focus is a portable, cross-platform, one-click browser session
with split routing and no system VPN installation.

## License

MIT. See [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
