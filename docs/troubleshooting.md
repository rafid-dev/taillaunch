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

TailLaunch cleans up temporary directories on normal close, but a crash,
forced termination, or browser file lock can prevent removal. Close any
remaining TailLaunch/Chromium process before cleaning up temporary data. Never
delete a persistent state directory unless you intend to sign in again.

## Need help?

For a normal bug, use the repository's bug-report template and include the
version, OS/architecture, frontend, browser, reproduction steps, and sanitized
logs. For a suspected vulnerability, follow [SECURITY.md](../SECURITY.md)
instead of opening a public issue.
