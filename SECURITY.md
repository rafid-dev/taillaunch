# Security

## Reporting

Please report security-sensitive issues privately to the repository owner rather
than opening a public issue with exploit details.

## Important operational notes

TailLaunch stores a Tailscale node identity in its configured state directory.
Anyone who obtains that state may be able to impersonate the node until it is
revoked or expires. Do not sync, publish, or share the state directory.

TailLaunch intentionally binds its proxy to loopback only and refuses to start a
proxy listener on a non-loopback address.
