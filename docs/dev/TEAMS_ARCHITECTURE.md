# Teams Integration - Architecture & How It Works

## Visual Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        omniStatus CLI                            │
│  $ ost set --status "Meeting" --emoji ":calendar:" --state away │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
         ┌─────────────────────────────┐
         │  Parse Configuration        │
         │  ~/.config/omnistatus/      │
         │        config.yaml          │
         ├─────────────────────────────┤
         │ token: "eyJ0eXA..."        │
         │ user_id: "12345-abcd..."   │
         └──────────────┬──────────────┘
                        │
                        ▼
         ┌─────────────────────────────┐
         │  Create PresenceUpdate      │
         ├─────────────────────────────┤
         │ Status: "Meeting"           │
         │ Emoji: ":calendar:"         │
         │ State: away                 │
         └──────────────┬──────────────┘
                        │
                        ▼
         ┌─────────────────────────────┐
         │  Map State to Teams          │
         │  Availability               │
         ├─────────────────────────────┤
         │ away → "Away"               │
         └──────────────┬──────────────┘
                        │
                        ▼
         ┌─────────────────────────────┐
         │  Make Two API Calls to      │
         │  Microsoft Graph API        │
         ├─────────────────────────────┤
         │ 1. setPresence()            │
         │ 2. setStatusMessage()       │
         └──────────────┬──────────────┘
                        │
         ┌──────────────┴──────────────┐
         │                             │
         ▼                             ▼
    ┌─────────────┐          ┌──────────────────┐
    │ POST        │          │ PATCH            │
    │ /me/presence│          │ /me/presence     │
    │ /setPresence│          │                  │
    │             │          │                  │
    │ {"availab   │          │ {"statusMessage":│
    │  ility":    │          │  {"content":     │
    │  "Away"}    │          │   ":calendar:... │
    └──────┬──────┘          └────────┬─────────┘
           │                          │
           └──────────────┬───────────┘
                          │
                          ▼
        ┌─────────────────────────────────┐
        │   Microsoft Graph API            │
        │   (microsoft.com)                │
        │                                  │
        │   Processes API calls,           │
        │   updates user profile,          │
        │   syncs to all Teams clients     │
        └──────────────┬────────────────────┘
                       │
      ┌────────────────┼────────────────┐
      │                │                │
      ▼                ▼                ▼
   Desktop App    Web Client      Mobile App
   
   "Away" 🟡      "Away" 🟡      "Away" 🟡
   Status:       Status:        Status:
   ":calendar:   ":calendar:    ":calendar:
    Meeting"     Meeting"       Meeting"
```

## Request-Response Flow

### Request 1: Set Presence/Availability

```http
POST https://graph.microsoft.com/v1.0/me/presence/setPresence
Authorization: Bearer {ACCESS_TOKEN}
Content-Type: application/json

{
  "availability": "Away",
  "activity": "InACall"
}
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": "...presence_id...",
  "availability": "Away",
  "activity": "InACall"
}
```

### Request 2: Set Custom Status Message

```http
PATCH https://graph.microsoft.com/v1.0/me/presence
Authorization: Bearer {ACCESS_TOKEN}
Content-Type: application/json

{
  "statusMessage": {
    "content": ":calendar: Meeting",
    "expiryMethod": "endDateTime",
    "expiry": null
  }
}
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": "...presence_id...",
  "presence": {...},
  "statusMessage": {
    "content": ":calendar: Meeting",
    "expiry": null
  }
}
```

## Code Flow in omniStatus

```go
// cmd/set.go - User runs: ost set --status "Meeting" --state away

func runSet(cmd *cobra.Command, args []string) error {
    ┌─────────────────────────────────────────────────┐
    │ 1. Load Config                                  │
    │    cfg, err := config.LoadConfig()              │
    │    → Reads ~/.config/omnistatus/config.yaml     │
    │    → Parses YAML into Config struct             │
    │    → Extracts token and user_id                 │
    └─────────────────────────────────────────────────┘
                      ↓
    ┌─────────────────────────────────────────────────┐
    │ 2. Create PresenceUpdate                        │
    │    update := PresenceUpdate{                    │
    │        Status: "Meeting",                       │
    │        Emoji: ":calendar:",                     │
    │        State: "away",                           │
    │    }                                             │
    └─────────────────────────────────────────────────┘
                      ↓
    ┌─────────────────────────────────────────────────┐
    │ 3. Create Platform Manager                      │
    │    manager := platform.NewManager()             │
    │    manager.Register(                            │
    │        platform.NewTeamsUpdater(cfg.Teams)      │
    │    )                                             │
    │    → Creates TeamsUpdater with token, user_id   │
    │    → Checks IsEnabled() → true if both fields   │
    └─────────────────────────────────────────────────┘
                      ↓
    ┌─────────────────────────────────────────────────┐
    │ 4. Call UpdateAll() (concurrent)                │
    │    manager.UpdateAll(ctx, update)               │
    │    → Launches goroutine for TeamsUpdater        │
    │    → Goroutine calls:                           │
    │        updater.UpdatePresence(ctx, update)      │
    └─────────────────────────────────────────────────┘
}

// platform/teams.go - TeamsUpdater.UpdatePresence()

func (t *TeamsUpdater) UpdatePresence(ctx context.Context, 
                                      update PresenceUpdate) error {
    ┌─────────────────────────────────────────────────┐
    │ 1. Validate                                     │
    │    if !t.IsEnabled() {                          │
    │        return "teams not enabled or missing..."  │
    │    }                                             │
    └─────────────────────────────────────────────────┘
                      ↓
    ┌─────────────────────────────────────────────────┐
    │ 2. Map State                                    │
    │    availability := t.mapStateToTeamsAvailability│
    │                        (update.State)           │
    │    // "away" → "Away"                           │
    └─────────────────────────────────────────────────┘
                      ↓
    ┌─────────────────────────────────────────────────┐
    │ 3. Build Status Message                         │
    │    statusMessage := ":calendar: Meeting"        │
    │    (combines emoji + status text)               │
    └─────────────────────────────────────────────────┘
                      ↓
    ┌─────────────────────────────────────────────────┐
    │ 4. Call setPresence()                           │
    │    t.setPresence(ctx, "Away")                   │
    │    → POST to /me/presence/setPresence           │
    │    → Sends availability: "Away"                 │
    │    → Sends activity: "InACall"                  │
    │    → Returns error if status != 200             │
    └─────────────────────────────────────────────────┘
                      ↓
    ┌─────────────────────────────────────────────────┐
    │ 5. Call setStatusMessage()                      │
    │    t.setStatusMessage(ctx, statusMessage)       │
    │    → PATCH to /me/presence                      │
    │    → Sends statusMessage with content           │
    │    → No expiry (expiry: null)                   │
    │    → Returns error if status != 200             │
    └─────────────────────────────────────────────────┘
                      ↓
    ┌─────────────────────────────────────────────────┐
    │ 6. Return to Manager                            │
    │    if err != nil {                              │
    │        send error to errChan                    │
    │    } else {                                     │
    │        send success to successChan              │
    │    }                                             │
    └─────────────────────────────────────────────────┘
}
```

## Configuration Structure

```yaml
teams:                          # Platform section
  enabled: true                 # Enable/disable
  token: "eyJ0eXAi..."         # Access token
  extra:                        # Platform-specific
    user_id: "12345..."         # Azure AD Object ID
```

### How Configuration Maps to Code

```go
// config/config.go - Config struct

type Config struct {
    Teams *PlatformConfig `yaml:"teams"`  // ← Parsed from YAML
}

type PlatformConfig struct {
    Enabled bool              `yaml:"enabled"`
    Token   string            `yaml:"token"`
    Extra   map[string]string `yaml:"extra"`  // ← For user_id
}

// platform/teams.go - TeamsUpdater initialization

func NewTeamsUpdater(cfg *config.PlatformConfig) *TeamsUpdater {
    userID := ""
    if cfg.Extra != nil {
        userID = cfg.Extra["user_id"]  // ← Extracted here
    }
    
    return &TeamsUpdater{
        enabled: cfg.Enabled,
        token:   cfg.Token,
        userID:  userID,
    }
}
```

## HTTP Communication Details

### Authentication

All HTTP requests use Bearer token authentication:

```
Authorization: Bearer {ACCESS_TOKEN}
```

Example:
```
Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6Imk2bEdrM0...
```

### Request Structure

```go
// From platform/teams.go

func (t *TeamsUpdater) doGraphRequest(ctx context.Context, 
                                      endpoint string, 
                                      method string, 
                                      payload map[string]interface{}) error {
    // 1. Marshal payload to JSON
    jsonData, err := json.Marshal(payload)
    
    // 2. Create HTTP request
    req, err := http.NewRequestWithContext(ctx, method, endpoint, 
                                           bytes.NewBuffer(jsonData))
    
    // 3. Set headers
    req.Header.Set("Authorization", "Bearer "+t.token)  // ← Token here
    req.Header.Set("Content-Type", "application/json")
    
    // 4. Execute request
    client := &http.Client{}
    resp, err := client.Do(req)
    
    // 5. Check response
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("Graph API error: status %d", resp.StatusCode)
    }
    
    return nil
}
```

## State Mapping Logic

```go
func (t *TeamsUpdater) mapStateToTeamsAvailability(state PresenceState) string {
    switch state {
    case StateActive:
        return "Available"      // Maps to green indicator 🟢
    case StateAway:
        return "Away"           // Maps to yellow indicator 🟡
    case StateDND:
        return "DoNotDisturb"   // Maps to red indicator 🔴
    default:
        return "Available"      // Default to available
    }
}
```

| omniStatus | Teams API | Teams UI | What Others See |
|-----------|-----------|----------|-----------------|
| `active` | `Available` | Status: "Available" | 🟢 Green dot |
| `away` | `Away` | Status: "Away" | 🟡 Yellow dot |
| `dnd` | `DoNotDisturb` | Status: "Do Not Disturb" | 🔴 Red dot |

## Emoji Handling

```go
// Status message construction
statusMessage := update.Status
if update.Emoji != "" {
    statusMessage = fmt.Sprintf("%s %s", update.Emoji, update.Status)
}

// Examples:
// Input:  Status: "Coffee", Emoji: ":coffee:"
// Output: ":coffee: Coffee"

// Input:  Status: "Deep work", Emoji: ":brain:"
// Output: ":brain: Deep work"

// Input:  Status: "In meeting", Emoji: ""
// Output: "In meeting"
```

## Error Handling Flow

```
UpdatePresence() called
    ↓
IsEnabled() check
    ├─ NO → Return "teams not enabled or token/user_id missing"
    └─ YES ↓
      mapStateToTeamsAvailability()
          ↓
      setPresence() call
          ├─ Error (network, 401, etc.) → Return wrapped error
          └─ Success ↓
        setStatusMessage() call
            ├─ Error (network, 401, etc.) → Return wrapped error
            └─ Success ↓
          Return nil (success)
```

## Concurrency Model

```go
// From platform/platform.go - Manager.UpdateAll()

manager.UpdateAll(ctx, update)

// For each platform (Teams in this case):
go func() {
    if err := teamsUpdater.UpdatePresence(ctx, update); err != nil {
        errChan <- fmt.Errorf("teams: %w", err)
    } else {
        successChan <- "teams"
    }
}()

// Collect results:
for i := 0; i < numPlatforms; i++ {
    select {
    case err := <-errChan:
        fmt.Printf("✗ Error: %v\n", err)
    case name := <-successChan:
        fmt.Printf("✓ Updated %s\n", name)
    case <-ctx.Done():
        return fmt.Errorf("context timeout")
    }
}
```

This means:
- Teams update happens in parallel with other platforms
- Even if Slack times out, Teams still completes
- All errors are collected and reported
- Context timeout prevents indefinite hangs

## Token Lifecycle

```
1. Get Token
   ├─ Via Azure Portal authentication
   ├─ Valid for ~1 hour
   └─ Stored in config.yaml

2. Use Token
   ├─ Passed in Authorization header
   ├─ Validated by Microsoft Graph API
   └─ If valid: returns 200 OK + response data

3. Token Expires
   ├─ After ~1 hour
   ├─ Next API call gets 401 Unauthorized
   └─ User must get new token

4. Refresh Token
   ├─ Run OAuth 2.0 client credentials flow
   ├─ Get new access_token
   └─ Update config.yaml with new token
```

## Example: Complete Request/Response Cycle

### User Command
```bash
$ ost set --status "In meeting" --emoji ":calendar:" --state away
```

### 1. Presence Request
```
POST https://graph.microsoft.com/v1.0/me/presence/setPresence
Authorization: Bearer eyJ0eXAi...
Content-Type: application/json

{
  "availability": "Away",
  "activity": "InACall"
}
```

Response:
```json
{
  "id": "12345678-1234-1234-1234-123456789abc",
  "availability": "Away",
  "activity": "InACall",
  "availabilityLastModifiedDateTime": "2024-01-15T10:30:00.000Z"
}
```

### 2. Status Message Request
```
PATCH https://graph.microsoft.com/v1.0/me/presence
Authorization: Bearer eyJ0eXAi...
Content-Type: application/json

{
  "statusMessage": {
    "content": ":calendar: In meeting",
    "expiryMethod": "endDateTime",
    "expiry": null
  }
}
```

Response:
```json
{
  "id": "12345678-1234-1234-1234-123456789abc",
  "presence": {
    "availability": "Away",
    "activity": "InACall"
  },
  "statusMessage": {
    "content": ":calendar: In meeting",
    "expiry": null,
    "expiryMethod": "endDateTime"
  }
}
```

### 3. Teams Client Updates
Within 30-60 seconds, all Teams clients show:
- Status indicator: 🟡 Away
- Custom status: ":calendar: In meeting"

### 4. Clear Command
```bash
$ ost clear
```

Sends:
```json
// setPresence
{
  "availability": "Available",
  "activity": "InACall"
}

// setStatusMessage
{
  "statusMessage": {
    "content": "",
    "expiryMethod": "endDateTime",
    "expiry": null
  }
}
```

Teams shows:
- Status indicator: 🟢 Available
- Custom status: (empty)
