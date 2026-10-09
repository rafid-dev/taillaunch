# Security Model

This page explains the user-facing boundaries of TailLaunch. The reporting
policy is in [SECURITY.md](../SECURITY.md).

## Session lifecycle

Each non-persistent launch uses private, per-run Tailscale state and a
dedicated browser profile. TailLaunch attempts to remove both when the session
closes. Forced termination, crashes, and operating-system file locks can leave
temporary files behind. At the next start TailLaunch removes disposable folders
left by crashed sessions once no running session holds them; until then the
leftover data, including the Tailscale node key, remains on disk. Folders left
by crashed sessions of versions before v0.2.1 (including v0.2.0) have no
session marker, are not removed automatically, and may need one-time manual
removal: close all TailLaunch windows, then delete `taillaunch-state-*` and
`taillaunch-browser-*` folders from `%TEMP%` on Windows or from `$TMPDIR` (or
`/tmp`) on macOS and Linux.

The GUI's **Remember me on this device** and the CLI's `--persist` explicitly
opt into reusable state. Persistent state contains a Tailscale node identity
and browser data and must be protected like a credential. `--portable` only
changes the location of persistent data.

## Authentication

Authentication is handed to the system browser. TailLaunch does not collect a
Tailscale password or log the short-lived sign-in URL.

With the default Tailscale control plane, TailLaunch only opens HTTPS sign-in
URLs hosted on `tailscale.com` or its subdomains (such as
`login.tailscale.com`), and rejects userinfo and lookalike hosts. With a custom
control server, that server controls the authentication destination and may
legitimately direct the browser to another HTTPS host, as Tailscale itself
allows. TailLaunch still requires HTTPS and rejects userinfo, but configuring a
custom control server places that authentication flow inside your trust
boundary. Use an `https://` control server URL; TailLaunch only opens HTTPS
sign-in pages.

## Network boundaries

- The local HTTP proxy listens on loopback (`127.0.0.1`) only.
- The loopback proxy is unauthenticated. Any process that can connect to its
  port can use it, including to reach your tailnet destinations. Processes
  running as the same user are already inside the trust boundary. On a shared
  machine, however, another local user account that discovers the port could
  use the proxy while your session is open.
- Tailnet destinations go through the embedded userspace Tailscale node.
- Other destinations use the machine's normal network connection.
- HTTPS uses a normal CONNECT tunnel; TailLaunch does not decrypt or replace
  TLS certificates.
- TailLaunch does not install a system VPN or modify global proxy settings.

Tailscale ACLs and the security of the operating system, browser, control
server, and local filesystem remain part of the environment. TailLaunch is not
a malware sandbox and cannot protect data on a machine already controlled by
an attacker.
