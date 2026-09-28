# Microsoft Teams Integration - Complete Summary

## Overview

The Teams integration in omniStatus updates your presence and custom status in Microsoft Teams through the **Microsoft Graph API**. It runs concurrently with other platform updates for minimal latency.

## What Gets Updated?

When you run `ost set`, omniStatus makes **two API calls** to Microsoft Graph:

1. **Availability Status** (your presence indicator)
   - Green 🟢 = Available
   - Yellow 🟡 = Away
   - Red 🔴 = Do Not Disturb

2. **Custom Status Message** (text shown in your profile)
   - Your status message
   - With emoji if provided
   - Example: ":calendar: In meeting"

Both are visible to other Teams users and sync across all Teams clients (desktop, web, mobile).

---

## Required Configuration

```yaml
teams:
  enabled: true
  token: "YOUR_MICROSOFT_GRAPH_ACCESS_TOKEN"
  extra:
    user_id: "YOUR_AZURE_AD_OBJECT_ID"
```

### Configuration Fields

| Field | Purpose | Example | Where to Get |
|-------|---------|---------|--------------|
| `enabled` | Turn on/off Teams | `true` or `false` | You decide |
| `token` | API authorization | `eyJ0eXAi...` (JWT) | Azure Portal auth flow |
| `user_id` | Your Azure AD ID | `12345-abcd-...` | Azure AD Users section |

### Why Both Token and User ID?

- **Token** = "Prove who you are" (authentication)
- **User ID** = "Update THIS person's status" (which user to modify)

Without user ID, the API wouldn't know whose status to update.

---

## How to Get Your Credentials

### 1. Get Your Access Token

**High-level steps:**
1. Go to Azure Portal
2. Register an app (get Client ID)
3. Create a client secret
4. Exchange credentials for an access token
5. Copy the token to your config

**Detailed steps** → See **TEAMS_SETUP.md**

**Quick command:**
```bash
curl -X POST "https://login.microsoftonline.com/YOUR_TENANT_ID/oauth2/v2.0/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "scope=https://graph.microsoft.com/.default" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "grant_type=client_credentials"
```

### 2. Get Your User ID

**Option A - Via Azure Portal (easiest):**
1. Go to Azure AD → Users
2. Click your name
3. Copy the Object ID (UUID format)

**Option B - Via your access token:**
```bash
curl -X GET "https://graph.microsoft.com/v1.0/me" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Response includes `id` field = your User ID.

---

## How It Works (Code Level)

### User Runs Command
```bash
ost set --status "In meeting" --emoji ":calendar:" --state away
```

### Code Flow

```
main.go
  ↓
cmd.Execute()
  ↓
set.RunE() // In cmd/set.go
  ├─ config.LoadConfig()           // Read ~/.config/omnistatus/config.yaml
  ├─ Create PresenceUpdate{         // Build update with:
  │    Status: "In meeting"         //   - status message
  │    Emoji: ":calendar:"          //   - emoji
  │    State: "away"                //   - state
  │  }
  ├─ manager.NewManager()           // Create platform manager
  ├─ manager.Register(              // Register Teams updater
  │    NewTeamsUpdater(cfg.Teams)   //   with config token & user_id
  │  )
  └─ manager.UpdateAll()            // Launch concurrent updates
      ↓
      go func() {                   // In goroutine:
        teamsUpdater.UpdatePresence()
          ├─ IsEnabled() check       // Verify token & user_id exist
          ├─ mapStateToTeamsAvailability()  // "away" → "Away"
          ├─ setPresence()           // POST /me/presence/setPresence
          │   {"availability": "Away"}
          ├─ setStatusMessage()      // PATCH /me/presence
          │   {"statusMessage": {
          │     "content": ":calendar: In meeting"
          │   }}
          └─ Send result to channel
      }
```

### API Calls Made

**Call 1: Set Availability**
```http
POST https://graph.microsoft.com/v1.0/me/presence/setPresence
Authorization: Bearer {YOUR_TOKEN}
Content-Type: application/json

{
  "availability": "Away",
  "activity": "InACall"
}
```

**Call 2: Set Status Message**
```http
PATCH https://graph.microsoft.com/v1.0/me/presence
Authorization: Bearer {YOUR_TOKEN}
Content-Type: application/json

{
  "statusMessage": {
    "content": ":calendar: In meeting",
    "expiryMethod": "endDateTime",
    "expiry": null
  }
}
```

### Result

Within 30-60 seconds:
- Your presence appears as "Away" (yellow dot) 🟡
- Your custom status shows ":calendar: In meeting"
- Visible to all Teams users
- Updates on desktop, web, and mobile clients

---

## State Mapping

```
Input State (from --state flag)  →  Teams Availability  →  User Sees
────────────────────────────────────────────────────────────────────
active                            →  Available           →  🟢 Green
away                              →  Away                →  🟡 Yellow
dnd                               →  DoNotDisturb        →  🔴 Red
```

Code:
```go
func (t *TeamsUpdater) mapStateToTeamsAvailability(state PresenceState) string {
    switch state {
    case StateActive:
        return "Available"
    case StateAway:
        return "Away"
    case StateDND:
        return "DoNotDisturb"
    default:
        return "Available"
    }
}
```

---

## Usage Examples

### Example 1: Simple Status
```bash
ost set --status "Coffee break" --state away
```
Result:
- Status: "Coffee break"
- Availability: Away (🟡)

### Example 2: Status with Emoji
```bash
ost set --status "In meeting" --emoji ":calendar:" --state away
```
Result:
- Status: ":calendar: In meeting"
- Availability: Away (🟡)

### Example 3: Full Command
```bash
ost set --status "Deep focus" --emoji ":brain:" --state dnd
```
Result:
- Status: ":brain: Deep focus"
- Availability: Do Not Disturb (🔴)

### Example 4: Active Status
```bash
ost set --status "Working" --emoji ":computer:" --state active
```
Result:
- Status: ":computer: Working"
- Availability: Available (🟢)

### Example 5: Clear Status
```bash
ost clear
```
Result:
- Status: (empty)
- Availability: Available (🟢)

---

## Important Details

### Token Expiration
- Access tokens expire after ~1 hour
- After expiration, you get `401 Unauthorized` errors
- You must get a new token using the OAuth flow
- Update your config file with the new token

### What Emoji Codes Work?
Standard Slack/Discord emoji codes work:
- `:coffee:` ☕
- `:calendar:` 📅
- `:computer:` 💻
- `:brain:` 🧠
- `:rocket:` 🚀
- See full list: https://www.webfx.com/tools/emoji-cheat-sheet/

### Status Message Limits
- Maximum 280 characters
- Can include any text and emojis
- No special formatting or markdown in Graph API

### Sync Timing
- Availability (🟢🟡🔴) updates instantly
- Custom status message syncs within 30-60 seconds
- All Teams clients sync automatically (desktop, web, mobile)

---

## Configuration Security

### File Permissions
omniStatus creates `~/.config/omnistatus/config.yaml` with **0600 permissions**:
- Only you (owner) can read and write
- No other users can access
- No group or world permissions

### Token Safety
- Tokens are never logged
- Tokens are never printed
- Tokens are only stored in config file
- Treat your token like a password

### Best Practices
1. Use a service account if possible (not personal account)
2. Rotate credentials every 6-12 months
3. Never commit config file to git
4. Keep client secret safe until you exchange for token
5. Delete client secrets from Azure Portal after use

---

## Troubleshooting

### "teams is not enabled or token/user_id is missing"

**Check:**
1. Is `enabled: true` set? (not `false`)
2. Is `token` populated? (not empty)
3. Is `user_id` in `extra:` section? (not empty)
4. Is YAML indentation correct? (spaces, not tabs)

**Fix:**
```yaml
teams:
  enabled: true
  token: "eyJ0eXAi..."     # Must not be empty
  extra:
    user_id: "12345-..."   # Must not be empty
```

### "Graph API returned status 401: Unauthorized"

**Cause:** Token has expired (older than ~1 hour)

**Fix:**
1. Get a new access token using OAuth flow
2. Update `token` in config.yaml
3. Verify token is valid: `curl -X GET "https://graph.microsoft.com/v1.0/me" -H "Authorization: Bearer YOUR_TOKEN"`

### Status doesn't appear in Teams

**Check:**
1. Wait 30-60 seconds (async sync)
2. Check Teams shows your presence indicator (🟢/🟡/🔴)
3. Verify token with: `curl -X GET "https://graph.microsoft.com/v1.0/me" -H "Authorization: Bearer YOUR_TOKEN"`
4. Check permission scopes in Azure Portal are granted with admin consent

### YAML parsing errors

**Cause:** Indentation issues (common!)

**Fix:**
```yaml
# ❌ WRONG - don't use tabs
teams:
	enabled: true    ← Tab character breaks YAML
	token: "..."

# ✓ CORRECT - use spaces
teams:
  enabled: true    ← 2 spaces
  token: "..."
```

### Permission denied errors

**Cause:** You don't have permission to grant API scopes

**Fix:**
- Contact your Azure AD administrator
- Ask them to grant `Presence.ReadWrite` and `User.Read` scopes to your app
- They need to click "Grant admin consent" in Azure Portal

---

## Verification Commands

### Test Your Token
```bash
curl -X GET "https://graph.microsoft.com/v1.0/me" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Should return your user info (200 OK).

### Check Current Presence
```bash
curl -X GET "https://graph.microsoft.com/v1.0/me/presence" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Shows:
```json
{
  "availability": "Available",
  "activity": "InACall",
  "statusMessage": {...}
}
```

### Manually Update Status (for testing)
```bash
curl -X PATCH "https://graph.microsoft.com/v1.0/me/presence" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "statusMessage": {
      "content": "Test",
      "expiryMethod": "endDateTime",
      "expiry": null
    }
  }'
```

---

## Concurrency & Performance

### How It Works
- Teams update runs in a **goroutine** (concurrent)
- Doesn't block other platforms (Slack, Discord)
- Uses **channels** to collect results
- **30-second timeout** per update
- If one platform fails, others still complete

### Performance
- Teams API latency: typically 200-500ms
- With 3 platforms: ~500-700ms total (concurrent)
- Much faster than sequential: ~1500-2000ms

### Error Handling
- If Teams fails: error reported to user, other platforms continue
- Multiple failures are aggregated into one error message
- Context timeout prevents indefinite hangs

---

## Additional Resources

| Document | Purpose |
|----------|---------|
| **TEAMS_SETUP.md** | Step-by-step setup guide (Azure Portal walkthrough) |
| **TEAMS_QUICK_REFERENCE.md** | Quick reference card, TL;DR version |
| **TEAMS_ARCHITECTURE.md** | Technical deep dive, code flows, request details |
| **config/config.example.yaml** | Example configuration file |
| **platform/teams.go** | Source code implementation |

## Quick Links

- [Azure Portal](https://portal.azure.com)
- [Microsoft Graph API Docs](https://learn.microsoft.com/en-us/graph/api/resources/presence)
- [Azure App Registration](https://learn.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app)
- [OAuth 2.0 Client Credentials Flow](https://learn.microsoft.com/en-us/azure/active-directory/develop/v2-oauth2-client-creds-grant-flow)

---

## Summary

✅ **What You Need:**
- Access Token (from Azure OAuth flow)
- User ID (your Azure AD Object ID)

✅ **What Gets Updated:**
- Your presence indicator (🟢/🟡/🔴)
- Your custom status message

✅ **How Often:**
- Instantly when you run `ost set`
- Visibility syncs within 30-60 seconds to all clients

✅ **Security:**
- OAuth tokens (not passwords)
- 0600 file permissions
- No logging or printing of sensitive data

❌ **Common Mistakes:**
- Forgetting admin consent in Azure Portal
- Using expired tokens
- YAML indentation errors
- Missing user_id in config

**Get started:** Follow **TEAMS_SETUP.md** for step-by-step instructions.
