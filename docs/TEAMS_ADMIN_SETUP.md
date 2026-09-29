# Microsoft Teams: Admin Setup

One person in your Azure AD tenant needs to do this **once**. It registers an
app that every teammate then signs into individually - you're not creating
per-user credentials, just enabling the tenant to use omniStatus at all.

If you're an individual user, not an admin, you want
[TEAMS_SETUP.md](TEAMS_SETUP.md) instead - hand this document to whoever
manages your Azure AD tenant.

## What you're setting up

omniStatus uses delegated OAuth 2.0 via the **device code flow**: no client
secret, no redirect/callback server, works fine over SSH. Each user signs in
with their own Microsoft account and gets their own access/refresh tokens -
this app registration just identifies omniStatus to Azure AD and requests the
right permissions.

## Steps

1. Go to [Azure Portal](https://portal.azure.com) → **Azure Active Directory**
   (Entra ID) → **App registrations** → **+ New registration**.
   - Name: `omniStatus` (or anything you like)
   - Supported account types: "Accounts in this organizational directory
     only" is fine unless you specifically need multi-tenant
   - Redirect URI: leave blank - the device code flow doesn't use one
2. **Authentication** → **Add a platform** → **Mobile and desktop
   applications** (you don't need to fill in a redirect URI on this screen).
   Then, on the same Authentication page, scroll to **Advanced settings**
   and explicitly set **Allow public client flows** to **Yes**, and **Save**.
   This second step is easy to miss and is the one that actually matters -
   adding the platform type alone does not reliably flip it. If it's left
   on "No", sign-in fails partway through with
   `AADSTS7000218: The request body must contain the following parameter:
   'client_assertion' or 'client_secret'` (confirmed live) - the device
   code is issued fine, but the token exchange at the end gets rejected
   because Azure still thinks this is a confidential client that needs a
   secret.
3. **API permissions** → **+ Add a permission** → **Microsoft Graph** →
   **Delegated permissions** (not "Application permissions" - the app-only
   flow doesn't work reliably with Graph's presence endpoints).
   Add:
   - `Presence.ReadWrite`
   - `User.Read`
4. Click **Grant admin consent for [Your Organization]**. This lets every
   user in the tenant sign in without seeing an individual consent prompt
   (or without needing their own admin-consent workflow, if your tenant's
   conditional access requires one). If you skip this, each user may hit a
   "needs admin approval" screen on their first device-code sign-in instead.
5. Go to **Overview** and copy two values:
   - **Application (client) ID**
   - **Directory (tenant) ID**

## Distributing access

Give every teammate who wants to use omniStatus these two values. That's the
entire per-user setup burden on your end - they do their own sign-in from
there (see TEAMS_SETUP.md). You never see or handle their tokens; each
user's access/refresh token pair is obtained directly between their device
and Microsoft, then stored only in their own local omniStatus config.

## Troubleshooting

**`AADSTS7000218: ... must contain ... 'client_assertion' or
'client_secret'`.** "Allow public client flows" is set to "No" on the app
registration - go to **Authentication → Advanced settings** and set it to
**Yes** (see step 2 above). This is the single most common setup mistake.

**A user reports "needs admin approval" during sign-in.** Admin consent
(step 4) wasn't granted, or was granted after they already hit the prompt.
Grant it from **API permissions**, or have them retry after you do.

**"Permissions need admin consent" / can't find `Presence.ReadWrite`.**
Some tenants restrict which Graph permissions are available. Confirm
Microsoft Graph API access isn't restricted in your tenant's enterprise
application settings.

**Conditional access policies blocking sign-in.** Device code flow is
sometimes specifically restricted by conditional access policies (it's a
common phishing vector for other apps, so some tenants disable it tenant-wide
or per-app). If users can't complete sign-in at all, check whether your
tenant has a policy blocking the device code grant type and exempt this
app's client ID if needed.

**Rotating/revoking access.** There's no per-app secret to rotate (public
client, no secret). To cut off a specific user, revoke their refresh token
from **Azure Portal → Users → [user] → Authentication methods**, or disable
the app registration entirely to cut off everyone at once.
