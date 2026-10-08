# GUI Usage

`taillaunch-gui` is TailLaunch's normal desktop launcher.

```bash
taillaunch-gui
```

## Connect and open an app

1. Start disconnected and click **Connect to Tailscale**.
2. Complete authentication on the official Tailscale page in the system
   browser.
3. After the launcher shows **Connected**, enter a tailnet hostname or a full
   `http://`/`https://` URL in **Tailnet address**.
4. Click **Open App**.

The private app opens maximized in a normal Chromium-family app window with
ordinary title-bar controls. Closing it ends the current connection session.

## Remember me

GUI sessions are temporary by default. **Remember me on this device** saves
the identity and browser profile locations so later launches can reuse them.
It is equivalent to the CLI's `--persist` behavior. Treat persistent state as
a credential; see [Security Model](security-model.md).

## Settings

The **Settings** screen keeps advanced options out of the main connection
flow:

- **Browser executable**: choose a specific Edge, Chrome, Brave, or Chromium
  executable; blank selects automatically.
- **Control server / Headscale URL**: use an alternate control server.
- **Memory mode**: **Auto** by default, or explicit **Normal** / **Low memory**.
- **Verbose logging**: enable additional local diagnostic logging.

See [Headscale](headscale.md) for alternate control-server setup.

## Screenshots

- [Disconnected launcher](screenshots/launcher-disconnected.png)
- [Connected launcher](screenshots/launcher-connected.png)
- [Settings](screenshots/settings.png)
