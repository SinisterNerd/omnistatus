# Microsoft Teams: User Setup

This gets your own Teams presence and status syncing through omniStatus.
It assumes someone in your organization has already registered the Azure
app (a one-time, tenant-wide step) - if nobody has, see
[TEAMS_ADMIN_SETUP.md](TEAMS_ADMIN_SETUP.md) first (or do it yourself if
you're the admin).

## 1. Get two values from your admin

Ask whoever manages your Azure AD tenant for:

- **Client ID** (the app registration's Application ID)
- **Tenant ID** (your organization's Directory ID)

These are the same for everyone in your organization - not personal
credentials. You'll get your own personal sign-in in step 3.

## 2. Add them to your config

Edit `~/.config/omnistatus/config.yaml`:

```yaml
teams:
  enabled: true
  extra:
    client_id: "THE_CLIENT_ID_YOUR_ADMIN_GAVE_YOU"
    tenant_id: "THE_TENANT_ID_YOUR_ADMIN_GAVE_YOU"
```

Leave `token` and `extra.refresh_token` out - they get filled in
automatically in the next step.

## 3. Sign in

Run any `ost` command, e.g.:

```bash
ost set --status "Testing Teams integration" --state active
```

Since `token`/`refresh_token` are missing, omniStatus automatically starts a
one-time device-code sign-in. You'll see something like:

```
═══════════════════════════════════════════════════════
  Microsoft Teams sign-in required
═══════════════════════════════════════════════════════
To sign in, use a web browser to open the page https://microsoft.com/devicelogin
and enter the code ABCD1234 to authenticate.
═══════════════════════════════════════════════════════
Waiting for you to complete sign-in...
```

Open that URL on any device (your phone is fine), enter the code, sign in
with your normal Microsoft/Teams account. omniStatus saves your
access/refresh tokens back into your config file automatically once you
complete it - no need to run this again unless the refresh token is later
revoked.

From then on, tokens refresh silently in the background (access tokens last
~1 hour; the refresh token handles renewal transparently), so this is a
one-time step per machine.

## What gets updated in Teams

| omniStatus state | Teams availability |
|---|---|
| `active` | Available |
| `away` | Away |
| `dnd` | DoNotDisturb |
| `busy` | Busy |
| `brb` | BeRightBack |
| `offline` | Offline |

`ost set --status "..."` also sets your Teams custom status message (the
text shown under your name), via Graph's `setStatusMessage` action. Two
real limitations, confirmed against the live API:

- **No emoji field** - Teams status messages are plain text only, so
  `--emoji` is a no-op for Teams (it still works for Slack/GitHub).
- **`--duration` doesn't apply to the status message** - Graph silently
  ignores an expiration on this specific endpoint, so a timed status only
  auto-clears your *availability*, not the status text. `ost clear` clears
  both immediately.

## Troubleshooting

**`graph API returned status 401: Unauthorized`.** Your access token
expired and the automatic refresh also failed - usually means the refresh
token was revoked (e.g. a password reset, or an admin revoking your
session). Delete `token` and `extra.refresh_token` from your config and
re-run any `ost` command to sign in again.

**Stuck on "needs admin approval" during sign-in.** Your organization's app
registration doesn't have admin consent granted yet, or your tenant has a
conditional access policy blocking device-code sign-in. This is on the
admin side - point them at TEAMS_ADMIN_SETUP.md's troubleshooting section.

**Status doesn't appear to update in the Teams client.** Graph API changes
can take 30-60 seconds to show up in a running Teams client. You can verify
the change actually landed with:
```bash
ost status --platform teams
```

**Wrong `client_id`/`tenant_id`.** Double-check the values against what
your admin gave you - a typo here usually surfaces as the device-code
request itself failing immediately, before you even get a code to enter.
