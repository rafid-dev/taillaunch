# CLI Usage

`taillaunch` is the command-line frontend.

## Open a private app

The first argument is an `http://` or `https://` URL with a host:

```bash
taillaunch https://my-server.my-tailnet.ts.net
```

The CLI uses a temporary Tailscale node state and browser profile by default.
When sign-in is needed, the official Tailscale page opens in the system
browser.

## Common options

```text
--persist       reuse saved Tailscale identity and browser data
--portable      with --persist, store data next to the executable
--app=false     use a full browser window instead of app mode
--low-memory    use conservative Chromium process and cache limits
--browser PATH  use a specific browser executable
--verbose       print additional local diagnostics
```

Use `taillaunch --help` for the complete flag list.

## Proxy-only mode

Use TailLaunch as a loopback proxy without opening a browser:

```bash
taillaunch --proxy-only
```

TailLaunch prints `HTTP_PROXY` and `HTTPS_PROXY` values. The proxy is bound to
`127.0.0.1` only.

## Portable persistent mode

```bash
taillaunch --persist --portable https://nas
```

This stores persistent state under `taillaunch-data/` next to the executable.
`--portable` alone does not make a session persistent.

## Alternate control server

```bash
taillaunch --control-url https://headscale.example.com https://nas
```

See [Headscale](headscale.md) for the corresponding GUI setting and notes.
