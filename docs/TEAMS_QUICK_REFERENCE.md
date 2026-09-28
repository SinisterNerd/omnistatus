# Teams Configuration - Quick Reference

## TL;DR - 5 Minute Setup

### What You Need
1. **Access Token** - API key from Microsoft
2. **User ID** - Your Azure AD object ID (UUID)

### Quick Setup Path

```bash
# 1. Go to https://portal.azure.com
# 2. Azure AD → App registrations → New registration → Name: "omniStatus"
# 3. API permissions → Add "Presence.ReadWrite" and "User.Read" → Grant admin consent
# 4. Certificates & secrets → New client secret → Copy value
# 5. Get access token:
curl -X POST "https://login.microsoftonline.com/YOUR_TENANT_ID/oauth2/v2.0/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "scope=https://graph.microsoft.com/.default" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "grant_type=client_credentials"

# 6. Get your User ID:
curl -X GET "https://graph.microsoft.com/v1.0/me" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"

# 7. Update config file: ~/.config/omnistatus/config.yaml
teams:
  enabled: true
  token: "YOUR_ACCESS_TOKEN_HERE"
  extra:
    user_id: "YOUR_OBJECT_ID_HERE"

# 8. Test it:
ost set --status "Testing Teams" --emoji ":test:" --state active
```

---

## Key Information

### What Gets Updated?

| Field | What It Does | Example |
|-------|-------------|---------|
| **Availability** | Your presence indicator (green/yellow/red) | `Available` / `Away` / `DoNotDisturb` |
| **Status Message** | Custom text shown in your profile | `:coffee: Coffee break` |

### State Mapping

```
omniStatus                 →  Teams
├─ active                  →  Available (🟢 green)
├─ away                    →  Away (🟡 yellow)
└─ dnd                     →  DoNotDisturb (🔴 red)
```

### Configuration Example

```yaml
teams:
  enabled: true
  token: "eyJ0eXAiOiJKV1QiLCJhbGc..."      # Access token (40+ chars)
  extra:
    user_id: "12345678-abcd-1234-abcd-..."  # UUID format
```

---

## Getting the Token (Step-by-Step)

### Find Your Tenant ID

In Azure Portal:
- Go to **Azure Active Directory** → **Properties**
- Copy the **Directory (tenant) ID**

### Create App Registration

1. **Azure AD** → **App registrations** → **+ New registration**
2. Name: `omniStatus`
3. Click **Register**

### Add Permissions

1. **API permissions** → **+ Add a permission**
2. Select **Microsoft Graph** → **Application permissions**
3. Search and add:
   - ✓ `Presence.ReadWrite`
   - ✓ `User.Read`
4. **Grant admin consent** (important!)

### Create Client Secret

1. **Certificates & secrets** → **+ New client secret**
2. Description: `omniStatus`
3. Expiration: 6 months
4. **Copy the value immediately** (you won't see it again)

### Exchange for Access Token

```bash
# Store these for clarity
TENANT_ID="<your tenant ID>"
CLIENT_ID="<your app client ID from Overview>"
CLIENT_SECRET="<the secret value you just copied>"

# Get the token
curl -X POST "https://login.microsoftonline.com/$TENANT_ID/oauth2/v2.0/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=$CLIENT_ID" \
  -d "scope=https://graph.microsoft.com/.default" \
  -d "client_secret=$CLIENT_SECRET" \
  -d "grant_type=client_credentials"
```

Response:
```json
{
  "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
  "expires_in": 3599,
  "token_type": "Bearer"
}
```

**Copy the `access_token` value** to your config.

---

## Getting Your User ID

### Option 1: Azure Portal (Easiest)

1. **Azure AD** → **Users**
2. Click on your username
3. Copy the **Object ID**

### Option 2: Using Your Access Token

```bash
curl -X GET "https://graph.microsoft.com/v1.0/me" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

Look for the `id` field in response:
```json
{
  "id": "12345678-1234-1234-1234-123456789012",  ← This is your User ID
  "userPrincipalName": "you@company.com",
  ...
}
```

---

## Configuration Checklist

- [ ] Access token obtained from Azure
- [ ] Access token starts with `eyJ0eXA` (JWT format)
- [ ] User ID is UUID format (8-4-4-4-12 hex digits)
- [ ] Config file created at `~/.config/omnistatus/config.yaml`
- [ ] `teams.enabled: true`
- [ ] `teams.token` populated
- [ ] `teams.extra.user_id` populated
- [ ] YAML indentation is correct (no tabs, use spaces)
- [ ] Tested with: `ost set --status "test" --state active`

---

## Usage Examples

```bash
# Update status with everything
ost set --status "In meeting" --emoji ":calendar:" --state away

# Just status and state
ost set --status "Deep work" --state dnd

# Just status
ost set --status "Coffee break"

# With default state (active)
ost set --status "Available" --emoji ":computer:"

# Clear everything
ost clear
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `teams is not enabled` | Set `enabled: true` in config |
| `token/user_id is missing` | Verify both fields are populated, check indentation |
| `401 Unauthorized` | Token expired, get a new one using curl command above |
| Status doesn't appear | Wait 30-60 seconds, Teams syncs asynchronously |
| Can't find permission scopes | Contact your Azure AD admin, may need to enable for org |
| YAML parsing error | Check indentation (spaces, not tabs) |

---

## Verify It Works

### Check Token Is Valid

```bash
curl -X GET "https://graph.microsoft.com/v1.0/me" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Should return your user info (200 OK), not an error.

### Check Current Presence

```bash
curl -X GET "https://graph.microsoft.com/v1.0/me/presence" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Returns something like:
```json
{
  "id": "abc123...",
  "availability": "Available",
  "activity": "InACall"
}
```

### Manually Set Status (test)

```bash
curl -X PATCH "https://graph.microsoft.com/v1.0/me/presence" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "statusMessage": {
      "content": "Test from curl",
      "expiryMethod": "endDateTime",
      "expiry": null
    }
  }'
```

---

## Important Notes

### Token Expiration
- Access tokens expire after ~1 hour
- When expired, you get a 401 error
- Get a new token using the curl command above
- Update your config file

### Security
- Never commit config file to git
- Ensure `~/.config/omnistatus/config.yaml` has 0600 permissions
- Treat client secret like a password
- Rotate credentials every 6-12 months

### Permissions
- `Presence.ReadWrite` - Allows updating your status
- `User.Read` - Allows reading your profile (needed to update)
- Both must be granted and have admin consent

---

## Common Mistakes

❌ **Wrong token format** - Should start with `eyJ0eXA`
❌ **Missing .extra.user_id** - Must be nested under `extra:`
❌ **Using tabs instead of spaces** - YAML requires spaces for indentation
❌ **Expired token** - Check if 1+ hour has passed since getting it
❌ **Forgetting admin consent** - Green "Grant admin consent" button must be clicked

---

## Need Help?

See **TEAMS_SETUP.md** for the full step-by-step guide with screenshots and detailed explanations.
