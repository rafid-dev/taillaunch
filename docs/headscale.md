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

TailLaunch still opens the official authentication page through the system
browser when the selected control server requires authorization. TailLaunch
does not validate or manage the control server's account policy; confirm that
the server, tailnet policy, DNS names, and certificates are configured for the
private app you want to open.

For persistence and network-boundary details, see the [Security Model](security-model.md).
