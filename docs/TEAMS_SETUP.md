# Microsoft Teams Configuration Guide

This guide explains how to set up Teams integration for omniStatus, including step-by-step instructions for obtaining the required credentials.

## Overview

The Teams integration uses **Microsoft Graph API** to update your presence and custom status message in Microsoft Teams. It requires two pieces of information:

1. **Access Token** - Authorization to use Microsoft Graph API
2. **User ID** - Your Azure AD object ID

## Configuration Structure

```yaml
teams:
  enabled: true
  token: "YOUR_MICROSOFT_GRAPH_ACCESS_TOKEN_HERE"
  extra:
    user_id: "YOUR_AZURE_AD_OBJECT_ID_HERE"
```

### Fields Explained

| Field | Required | Type | Purpose |
|-------|----------|------|---------|
| `enabled` | Yes | boolean | Enable/disable Teams integration |
| `token` | Yes | string | Microsoft Graph API access token |
| `extra.user_id` | Yes | string | Your Azure AD Object ID (UUID) |

## What Gets Updated in Teams

When you run `ost set`, omniStatus updates:

1. **Availability Status** - Your presence state shown to others
   - `Available` (from `active` state)
   - `Away` (from `away` state)
   - `DoNotDisturb` (from `dnd` state)

2. **Custom Status Message** - Your profile status text
   - Shows your status message
   - Includes emoji if provided
   - Example: `:coffee: Coffee break`

## Step-by-Step Setup

### Phase 1: Register an Application in Azure

#### Step 1: Go to Azure Portal

1. Visit https://portal.azure.com
2. Sign in with your Microsoft account (same one linked to Teams)
3. Navigate to **Azure Active Directory** from the sidebar

#### Step 2: Register a New Application

1. Click **App registrations** in the left sidebar
2. Click **+ New registration**
3. Fill in the form:
   - **Name**: `omniStatus` (or your preferred name)
   - **Supported account types**: Select "Accounts in this organizational directory only" (default)
   - Leave **Redirect URI** empty for now
4. Click **Register**

#### Step 3: Configure API Permissions

1. You'll be taken to your app's overview page
2. Click **API permissions** in the left sidebar
3. Click **+ Add a permission**
4. Select **Microsoft Graph** from the list
5. Select **Application permissions** (NOT "Delegated permissions")
6. Search for and add these permissions:
   - `Presence.ReadWrite` - Read/write presence status
   - `User.Read` - Read user profile information
7. Click **Add permissions**
8. **Important**: Click **Grant admin consent for [Your Organization]**
   - You may need admin rights or need to ask your admin to do this
   - This button will turn green once granted

![Azure Permissions Setup](https://example.com/azure-permissions.png)

#### Step 4: Create a Client Secret

1. Click **Certificates & secrets** in the left sidebar
2. Click **+ New client secret**
3. Add a description: `omniStatus key`
4. Select expiration: **6 months** or longer
5. Click **Add**
6. **IMPORTANT**: Copy the secret value immediately (you won't see it again!)
   - Save it somewhere secure temporarily
   - You'll need it in the next phase

### Phase 2: Get Your Access Token

#### Step 1: Get Your Client ID and Secret

From the same app registration page:

1. Click **Overview** in the left sidebar
2. Copy your **Application (client) ID** - you'll need this
3. Go back to **Certificates & secrets** and verify your secret is visible

#### Step 2: Obtain Access Token

You have two options:

**Option A: Using PowerShell (Windows)**

```powershell
$tenantId = "YOUR_TENANT_ID"        # From Azure > Directory properties
$clientId = "YOUR_CLIENT_ID"         # From app registration
$clientSecret = "YOUR_CLIENT_SECRET" # From Certificates & secrets

$body = @{
    Grant_Type    = "client_credentials"
    Scope         = "https://graph.microsoft.com/.default"
    Client_Id     = $clientId
    Client_Secret = $clientSecret
}

$response = Invoke-RestMethod -Uri "https://login.microsoftonline.com/$tenantId/oauth2/v2.0/token" -Method POST -Body $body

$accessToken = $response.access_token
Write-Host $accessToken
```

**Option B: Using curl (macOS/Linux)**

```bash
TENANT_ID="YOUR_TENANT_ID"
CLIENT_ID="YOUR_CLIENT_ID"
CLIENT_SECRET="YOUR_CLIENT_SECRET"

curl -X POST "https://login.microsoftonline.com/$TENANT_ID/oauth2/v2.0/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=$CLIENT_ID" \
  -d "scope=https://graph.microsoft.com/.default" \
  -d "client_secret=$CLIENT_SECRET" \
  -d "grant_type=client_credentials"
```

**Option C: Using Postman**

1. Open Postman
2. Create a new POST request
3. URL: `https://login.microsoftonline.com/{TENANT_ID}/oauth2/v2.0/token`
4. Body (x-www-form-urlencoded):
   - `client_id`: Your Client ID
   - `scope`: `https://graph.microsoft.com/.default`
   - `client_secret`: Your Client Secret
   - `grant_type`: `client_credentials`
5. Send and copy the `access_token` from response

The response will look like:
```json
{
  "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
  "expires_in": 3599,
  "ext_expires_in": 3599,
  "token_type": "Bearer"
}
```

**Copy the `access_token` value** - this is what goes in your config.

### Phase 3: Get Your Azure AD Object ID

#### Method 1: Via Azure Portal (Easiest)

1. In Azure Portal, go to **Azure Active Directory**
2. Click **Users**
3. Find your username and click it
4. Copy the **Object ID** (it's a UUID format like `12345678-1234-1234-1234-123456789012`)

#### Method 2: Via Microsoft Graph API

```bash
ACCESS_TOKEN="YOUR_TOKEN_FROM_PHASE_2"

curl -X GET "https://graph.microsoft.com/v1.0/me" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json"
```

Response will include:
```json
{
  "id": "12345678-1234-1234-1234-123456789012",
  "userPrincipalName": "user@company.com",
  "displayName": "Your Name",
  ...
}
```

Copy the `id` field - this is your User ID.

#### Method 3: Via PowerShell

```powershell
Connect-MgGraph -Scopes "User.Read"
Get-MgUser -UserId "me" | Select-Object Id
```

### Phase 4: Update Your Config

Now you have all three pieces of information:

```yaml
teams:
  enabled: true
  token: "eyJ0eXAiOiJKV1QiLCJhbGc..."  # Access token from Phase 2
  extra:
    user_id: "12345678-1234-1234-1234-123456789012"  # Object ID from Phase 3
```

## State Mapping

omniStatus converts availability states as follows:

| omniStatus State | Teams Availability | What Others See |
|------------------|------------------|-----------------|
| `active` | `Available` | Green presence indicator |
| `away` | `Away` | Yellow presence indicator |
| `dnd` | `DoNotDisturb` | Red presence indicator |

**Example usage:**

```bash
# Shows as Available in Teams
ost set --status "Working on feature X" --state active

# Shows as Away in Teams
ost set --status "Coffee break" --emoji ":coffee:" --state away

# Shows as Do Not Disturb in Teams
ost set --status "In deep focus" --emoji ":brain:" --state dnd
```

## Microsoft Graph API Endpoints Used

omniStatus uses these two endpoints:

### 1. Set Presence/Availability

**Endpoint**: `POST /v1.0/me/presence/setPresence`

```http
POST https://graph.microsoft.com/v1.0/me/presence/setPresence
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "availability": "Available",
  "activity": "InACall"
}
```

### 2. Set Custom Status Message

**Endpoint**: `PATCH /v1.0/me/presence`

```http
PATCH https://graph.microsoft.com/v1.0/me/presence
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "statusMessage": {
    "content": ":coffee: Coffee break",
    "expiryMethod": "endDateTime",
    "expiry": null
  }
}
```

## Token Refresh & Expiration

**Important**: Access tokens expire after a certain period (typically 1 hour).

### Automatic Refresh Strategy (Future Enhancement)

Currently, omniStatus does **not** automatically refresh tokens. When your token expires, you'll get an error like:

```
Graph API returned status 401: Unauthorized
```

### How to Handle Token Expiration

1. **Short-term**: Get a new token using the Phase 2 procedure and update your config
2. **Long-term**: Implement token refresh logic (see dev/DEVELOPMENT.md)

To check if your token is valid:

```bash
curl -X GET "https://graph.microsoft.com/v1.0/me" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

If you get 200 OK with your user data, the token is valid.
If you get 401 Unauthorized, the token has expired.

## Troubleshooting

### Error: "Graph API returned status 401: Unauthorized"

**Cause**: Access token has expired or is invalid

**Solution**:
1. Get a new access token using Phase 2 procedure
2. Update `token` in your config file
3. Verify using: `curl -X GET "https://graph.microsoft.com/v1.0/me" -H "Authorization: Bearer YOUR_TOKEN"`

### Error: "teams is not enabled or token/user_id is missing"

**Cause**: Configuration is incomplete

**Solution**:
1. Verify `enabled: true` is set
2. Verify `token` is not empty
3. Verify `user_id` in `extra` section is not empty
4. Check for YAML indentation (very common issue!)

Example of correct indentation:
```yaml
teams:
  enabled: true
  token: "your_token_here"
  extra:
    user_id: "your_id_here"
```

### Error: "Permissions need admin consent"

**Cause**: You don't have permission to grant API access

**Solution**:
1. Ask your organization's Azure AD administrator to:
   - Go to Azure Portal > Azure Active Directory > App registrations
   - Find your app
   - Go to API permissions
   - Click "Grant admin consent for [Organization]"
2. Or request that they grant `Presence.ReadWrite` and `User.Read` permissions

### Status doesn't update in Teams desktop app immediately

**Normal behavior**: Graph API changes can take 30-60 seconds to appear in the Teams client

**Workaround**: 
- You can still verify it worked with `curl`:
```bash
curl -X GET "https://graph.microsoft.com/v1.0/me/presence" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Can't find "Presence.ReadWrite" permission

**Cause**: Your Azure subscription or admin hasn't enabled Graph API for your organization

**Solution**:
1. Contact your Azure AD administrator
2. Ask them to ensure Microsoft Graph API is available
3. They should be able to grant the permissions manually

## Security Best Practices

1. **Keep token private**
   - Don't share your token
   - Don't commit to git
   - Ensure config file has 0600 permissions

2. **Minimize token lifetime**
   - When creating the client secret, use 6-month expiration
   - Refresh tokens regularly

3. **Rotate credentials**
   - Every 6-12 months, create a new client secret
   - Delete the old one from Azure Portal

4. **Limit scopes**
   - Only grant `Presence.ReadWrite` and `User.Read`
   - Don't grant broader permissions than needed

## Reference Information

### Availability States

Teams supports these availability states in Graph API:

| State | Display | Description |
|-------|---------|-------------|
| `Available` | 🟢 Green | User is available |
| `AvailableIdle` | 🟢 Green | Idle but available |
| `Away` | 🟡 Yellow | Away from desk |
| `BeRightBack` | 🟡 Yellow | Will be right back |
| `Busy` | 🔴 Red | Busy |
| `BusyIdle` | 🔴 Red | Idle but busy |
| `DoNotDisturb` | 🔴 Red | Do Not Disturb |
| `Offline` | ⚫ Offline | Offline |

omniStatus maps:
- `active` → `Available`
- `away` → `Away`
- `dnd` → `DoNotDisturb`

### Activities

When setting presence, omniStatus uses `InACall` activity, which indicates the user is actively working.

### Custom Status Message

Custom status messages:
- Maximum 280 characters
- Can include emojis
- Don't expire by default (we set `expiryMethod: null`)
- Updated instantly in Teams

## Example Workflow

```bash
# 1. Set up config with token and user_id
# (follow Phase 1-4 above)

# 2. Test with a simple update
ost set --status "Testing Teams integration" --state active

# 3. Check Teams desktop app - you should see:
#    - Your status as "Available" (green dot)
#    - Your custom status: "Testing Teams integration"

# 4. Try with emoji
ost set --status "Coffee break" --emoji ":coffee:" --state away
#    - Your status as "Away" (yellow dot)
#    - Your custom status: ":coffee: Coffee break"

# 5. Clear it
ost clear
#    - Your status as "Available" (default)
#    - Your custom status cleared
```

## FAQ

**Q: Can multiple applications access my Teams status?**
A: Yes, the token grants access to the Graph API, which controls all presence APIs. Only one change can happen at a time.

**Q: Does this work with Teams on mobile?**
A: Yes! Since it updates your Teams profile through the Graph API, all Teams clients (desktop, web, mobile) will reflect the changes.

**Q: Does this work with Teams in a browser?**
A: Yes, Graph API updates apply to all Teams clients.

**Q: What if my organization has conditional access policies?**
A: You may need to whitelist the Graph API or ensure your setup meets your organization's security requirements. Contact your IT department.

**Q: Can I have multiple omniStatus configs for different Microsoft accounts?**
A: Currently, omniStatus supports one configuration per user. You'd need to update the config file when switching accounts.

**Q: Do I need to keep the app secret safe?**
A: Yes! Treat it like a password. Never commit it to git, never share it. It only needs to stay safe until you get the access token.

**Q: How long is the access token valid?**
A: Typically 1 hour. After that, you need to get a new one.

**Q: Can I use a user token instead of app credentials?**
A: The current implementation uses app credentials (client secret). User delegation could be added in future versions for a different authentication flow.

## Related Resources

- [Microsoft Graph Presence API Documentation](https://learn.microsoft.com/en-us/graph/api/resources/presence)
- [Azure App Registration Guide](https://learn.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app)
- [OAuth 2.0 Client Credentials Flow](https://learn.microsoft.com/en-us/azure/active-directory/develop/v2-oauth2-client-creds-grant-flow)
- [Microsoft Teams Developer Portal](https://dev.teams.microsoft.com/)
