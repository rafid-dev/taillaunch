# Troubleshooting

## No supported browser found

Install Microsoft Edge, Google Chrome, Brave, or Chromium. If multiple
browsers are installed, choose one explicitly in GUI **Settings** or with the
CLI `--browser` flag.

## The sign-in page does not open

Check that the system browser is available and that the sign-in URL was not
blocked by a browser policy. Open the launcher again and enable verbose
logging only when more local diagnostics are needed. Never share sign-in URLs
or persistent state in a report.

## Connection fails

Check internet access, the Tailscale account's access to the target, and any
configured control-server URL. For Headscale, confirm the server is reachable
and its certificates and DNS configuration are valid.

## The app does not open

Use an `http://` or `https://` URL with a host in the CLI. In the GUI, a
hostname such as `nas` is accepted and treated as HTTPS. Confirm that the
target browser is not already holding the selected persistent profile open.

## A previous session left data behind

TailLaunch cleans up its temporary directories on normal close, but a crash,
forced termination, or browser file lock can prevent removal. The next time
TailLaunch starts, it automatically removes the `taillaunch-state-*` and
`taillaunch-browser-*` folders that crashed sessions left in the system
temporary directory, once no running TailLaunch session holds them. If a
folder cannot be removed (for example because a browser process still has files
open), TailLaunch logs the failure, carries on, and tries again at the next
start. Close any remaining TailLaunch/Chromium process and start TailLaunch
again, or delete the folder yourself. Never delete a persistent state directory
unless you intend to sign in again.

Folders left by crashed sessions of versions before v0.2.1 (including v0.2.0)
carry no marker, so they are **not** removed automatically; TailLaunch cannot
tell whether such a folder still belongs to a running session. They may need
one-time manual removal. Close all TailLaunch windows and any browser windows
TailLaunch opened first, then delete the folders named `taillaunch-state-*` and
`taillaunch-browser-*` (followed by digits) from the temporary directory:

- Windows: `%TEMP%`, usually `C:\Users\<you>\AppData\Local\Temp`
- macOS and Linux: `$TMPDIR`, or `/tmp` when it is unset (on macOS `$TMPDIR` is
  a per-user folder under `/var/folders`)

The `taillaunch-state-*` folders contain `tailscaled.state`, the Tailscale node
key.

## Need help?

For a normal bug, use the repository's bug-report template and include the
version, OS/architecture, frontend, browser, reproduction steps, and sanitized
logs. For a suspected vulnerability, follow [SECURITY.md](../SECURITY.md)
instead of opening a public issue.
