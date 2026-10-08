# Getting Started

TailLaunch opens private tailnet web apps in a dedicated Chromium-family app
window. It uses an embedded userspace Tailscale node and a loopback proxy; it
does not install a system VPN or TUN adapter.

## Requirements

- Windows, Linux, or macOS on `amd64` or `arm64`.
- Microsoft Edge, Google Chrome, Brave, or Chromium.
- A Tailscale account with access to the private app, or a compatible
  alternate control server.

## Recommended path: GUI

Run `taillaunch-gui`, click **Connect to Tailscale**, and finish sign-in on
the official Tailscale page in the system browser. Once connected, enter the
private app hostname or URL and click **Open App**.

The GUI starts with a temporary session. Enable **Remember me on this device**
only when saved identity and browser data should be reused on this device.

See [GUI usage](gui.md) for settings and the complete flow.

## CLI path

For scripts or terminal use:

```bash
taillaunch https://my-server.my-tailnet.ts.net
```

See [CLI usage](cli.md) for persistence, portable mode, low-memory mode, and
proxy-only operation.

## First connection

TailLaunch checks for a supported browser before starting sign-in. When
Tailscale needs authorization, the official sign-in page opens in the system
browser. TailLaunch resumes automatically after authentication and does not
handle or log your Tailscale password.

## Session data

Temporary Tailscale state and the dedicated browser profile are removed when a
session closes, subject to normal operating-system file-lock and forced-exit
limitations. Persistence is opt-in through the GUI checkbox or CLI
`--persist`. Read the [security model](security-model.md) before enabling it.
