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

GUI sessions are temporary by default. **Remember me on this device** keeps
the Tailscale identity and browser profile between launches so later launches
can reuse them. It is equivalent to the CLI's `--persist` behavior.

`settings.json` stores only the remember-me flag and the other options from the
Settings screen. It does not store the identity or the browser profile; those
live in fixed folders described below.

### Where persistent data lives

With **Remember me** on (or CLI `--persist` without `--portable`), TailLaunch
keeps its data in a `TailLaunch` folder in your per-user configuration
directory:

| OS | Folder |
| --- | --- |
| Windows | `%AppData%\TailLaunch` (usually `C:\Users\<you>\AppData\Roaming\TailLaunch`) |
| macOS | `~/Library/Application Support/TailLaunch` |
| Linux | `$XDG_CONFIG_HOME/TailLaunch`, or `~/.config/TailLaunch` when unset |

Inside it:

- `tailscale-state/` holds the Tailscale node identity (`tailscaled.state`,
  which contains the node key).
- `browser/` holds the dedicated Chromium profile (cookies, saved sessions, and
  cache for the apps you opened).
- `settings.json` holds the launcher settings.

CLI `--portable` uses `taillaunch-data/` next to the executable instead, and
`--state-dir` / `--profile-dir` choose other folders.

**Treat this data as a credential.** Anyone who can read `tailscale-state/` can
act as this device on your tailnet, and anyone who can read `browser/` can
reuse your signed-in web sessions. Do not sync, back up to shared storage,
publish, or share these folders. See [Security Model](security-model.md).

### Forgetting this device

Deleting local files does not revoke the device on the control server, so do
both:

1. Quit TailLaunch and any browser window it opened, and turn off **Remember me
   on this device**.
2. Delete `tailscale-state/` and `browser/` (or the whole `TailLaunch` folder to
   reset settings too).
3. Remove the device from the tailnet: in the Tailscale admin console, open
   **Machines**, choose the device's menu, and select **Remove**. On Headscale,
   delete the node (for example `headscale nodes delete`).

Until step 3, the old node key remains valid wherever a copy of it exists.

## Settings

The **Settings** screen keeps advanced options out of the main connection
flow:

- **Browser executable**: choose a specific Edge, Chrome, Brave, or Chromium
  executable; blank selects automatically.
- **Control server / Headscale URL**: use an alternate control server. Use an
  `https://` URL; see [Headscale](headscale.md).
- **Memory mode**: **Auto** by default, or explicit **Normal** / **Low memory**.
- **Verbose logging**: enable additional local diagnostic logging.

See [Headscale](headscale.md) for alternate control-server setup.

## Screenshots

- [Disconnected launcher](screenshots/launcher-disconnected.png)
- [Connected launcher](screenshots/launcher-connected.png)
- [Settings](screenshots/settings.png)
