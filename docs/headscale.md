# Headscale and Alternate Control Servers

TailLaunch can use an alternate Tailscale-compatible control server such as
Headscale.

## CLI

Pass the control-server URL with `--control-url`:

```bash
taillaunch --control-url https://headscale.example.com https://nas
```

Combine it with `--persist` when the node identity should survive future
launches.

## GUI

Open **Settings**, enter the server in **Control server / Headscale URL**, save
settings, and then connect. Leave the field blank to use the normal Tailscale
control server.

## Authentication and scope

Use an `https://` control server URL. TailLaunch only opens HTTPS sign-in
pages, so a control server that is reachable only over plain `http://` cannot
complete browser sign-in.

TailLaunch opens the sign-in page through the system browser when the selected
control server requires authorization. With a custom control server, that server
decides where authentication happens and may send the browser to a different
HTTPS host (for example an external identity provider), as Tailscale itself
allows. TailLaunch still requires HTTPS and rejects userinfo, but the sign-in
host is not restricted to `tailscale.com`, so this authentication flow is inside
your trust boundary: only use a control server you trust. (With the default
Tailscale control server, only `tailscale.com` hosts are opened.)

TailLaunch does not validate or manage the control server's account policy; confirm that
the server, tailnet policy, DNS names, and certificates are configured for the
private app you want to open.

For persistence and network-boundary details, see the [Security Model](security-model.md).
