# Security

## Reporting a vulnerability privately

Please do not open a public issue with security-sensitive details. Use
GitHub's private vulnerability reporting or security advisory mechanism for
this repository. If private reporting is unavailable, contact the maintainers
through a private channel rather than publishing exploit details.

Include the affected TailLaunch version, operating system and architecture,
frontend (`taillaunch` or `taillaunch-gui`), browser, reproduction steps, and
impact. Remove passwords, short-lived sign-in URLs, node state, private host
names, and other sensitive data from reports and logs.

TailLaunch is a community project. There is no bug bounty, response-time SLA,
or promise that every report can be fixed. Reports are assessed in good faith
and coordinated fixes may be published through the normal release process.

## Supported releases

`v0.2.0` is the current public release line. Prefer the latest published
release when reporting or investigating a problem. Security fixes are
evaluated against the current release line and the default branch; older
versions may not receive separate fixes.

## Security model

### Disposable sessions by default

The GUI and CLI use temporary Tailscale node state and a temporary Chromium
profile unless persistence is explicitly enabled. TailLaunch attempts to
remove this per-run data when the session closes. A crash, forced termination,
or operating-system file lock can prevent cleanup, so users should treat the
local machine as part of the security boundary. At the next start TailLaunch
removes disposable folders left behind by crashed sessions once no running
session holds them. Until then, the leftover data, including the Tailscale node
key, remains on disk. Folders left by crashed sessions of versions before v0.2.1
(including v0.2.0) have no session marker, are not removed automatically, and
may need one-time manual removal: close all TailLaunch windows, then delete
`taillaunch-state-*` and `taillaunch-browser-*` folders from `%TEMP%` on
Windows or from `$TMPDIR` (or `/tmp`) on macOS and Linux.

The GUI's **Remember me on this device** option and the CLI's `--persist` flag
opt into reuse across launches. Persistent state includes a Tailscale node
identity and browser data; protect it like a credential and do not sync,
publish, or share it. `--portable` changes where persistent data is stored but
does not enable persistence by itself.

### Authentication

When authentication is needed, TailLaunch opens the sign-in page in the system
browser. TailLaunch does not collect a Tailscale password, embed a password
form, or write the short-lived sign-in URL to its own logs.

- **Default Tailscale control plane** (no control server set, or Tailscale's own
  control URL): TailLaunch only opens HTTPS sign-in URLs on `tailscale.com` or
  its subdomains (such as `login.tailscale.com`), without userinfo. Lookalikes
  such as `tailscale.com.evil.example` and `eviltailscale.com` are refused. This
  matches Tailscale's own check for the official control plane.
- **Custom control server** (for example Headscale): the control server decides
  where authentication happens, and Tailscale itself allows it to send the
  browser to a different HTTPS host, for example an external identity provider.
  TailLaunch still requires HTTPS, a host name, and no userinfo, but does not
  require the sign-in host to match the control server. Configuring a custom
  control server places that authentication flow inside your trust boundary, so
  only use servers you trust. Use an `https://` control server URL: TailLaunch
  only opens HTTPS sign-in pages.

Because TailLaunch also disables Tailscale's diagnostic log upload (see below),
the sign-in URL is not sent to Tailscale's log service or kept in the embedded
node's local log buffer.

### Tailscale diagnostic logs

The embedded Tailscale node (`tsnet`) normally uploads diagnostic logs to
`log.tailscale.com`, even when a custom control server such as Headscale is
configured. TailLaunch disables that upload (`logtail.Disable()` and
`TS_NO_LOGS_NO_SUPPORT=true`) before the node starts, for both Tailscale and
Headscale control servers. On startup with a persistent state directory it
also deletes log files left by earlier runs (`tailscaled.log.conf`,
`tailscaled.log1.txt`, `tailscaled.log2.txt`); it never touches
`tailscaled.state`.

The trade-off is that Tailscale support cannot access logs from TailLaunch's
embedded node to help diagnose problems.

### Network boundaries

- The local HTTP proxy binds to loopback only (`127.0.0.1`) and refuses a
  non-loopback listener.
- The loopback proxy is unauthenticated. Any process that can connect to its
  port can use it, including to reach your tailnet destinations. Processes
  running as the same user are already inside the trust boundary. On a shared
  machine, however, another local user account that discovers the port could
  use the proxy while your session is open.
- Tailnet destinations use the embedded userspace Tailscale node. Other
  destinations use the machine's normal network connection.
- HTTPS uses a normal CONNECT tunnel. TailLaunch does not intercept, decrypt,
  or replace TLS; the browser validates the destination certificate
  end-to-end.
- TailLaunch does not install a system VPN or TUN adapter and does not change
  the operating system's global proxy settings.

Tailscale ACLs, the selected control server, the operating system, the
installed browser, and the local filesystem remain outside TailLaunch's
control. TailLaunch does not provide a sandbox against malware or other users
who already control the machine.
