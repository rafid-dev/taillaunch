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
local machine as part of the security boundary.

The GUI's **Remember me on this device** option and the CLI's `--persist` flag
opt into reuse across launches. Persistent state includes a Tailscale node
identity and browser data; protect it like a credential and do not sync,
publish, or share it. `--portable` changes where persistent data is stored but
does not enable persistence by itself.

### Authentication

When authentication is needed, TailLaunch opens the official Tailscale sign-in
page in the system browser. TailLaunch does not collect a Tailscale password,
embed a password form, or write the short-lived sign-in URL to its logs.

### Network boundaries

- The local HTTP proxy binds to loopback only (`127.0.0.1`) and refuses a
  non-loopback listener.
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
