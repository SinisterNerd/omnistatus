# Microsoft Teams Integration - Documentation Index

This index guides you to the right document for your needs.

## Quick Navigation

| Need | Document | Read Time |
|------|----------|-----------|
| **Just the essentials** | [TEAMS_QUICK_REFERENCE.md](TEAMS_QUICK_REFERENCE.md) | 5 min ⚡ |
| **Step-by-step setup** | [TEAMS_SETUP.md](TEAMS_SETUP.md) | 15 min 📋 |
| **Complete overview** | [TEAMS_SUMMARY.md](TEAMS_SUMMARY.md) | 10 min 📖 |
| **How it works internally** | [TEAMS_ARCHITECTURE.md](TEAMS_ARCHITECTURE.md) | 15 min 🔧 |
| **Code implementation** | [platform/teams.go](platform/teams.go) | 10 min 💻 |

---

## Document Purposes

### 🚀 TEAMS_QUICK_REFERENCE.md
**For:** Users who want to get started fast

**Contains:**
- TL;DR 5-minute setup path
- Commands with examples
- Common mistakes to avoid
- Troubleshooting quick fixes
- Configuration checklist

**Best for:** "Just tell me what to do"

---

### 📋 TEAMS_SETUP.md
**For:** Users following a complete step-by-step guide

**Contains:**
- Phase 1: Azure Portal registration
- Phase 2: Getting access token
- Phase 3: Finding your User ID
- Phase 4: Updating config
- Detailed troubleshooting
- API endpoint reference
- Security best practices
- FAQ section

**Best for:** "Walk me through this carefully"

---

### 📖 TEAMS_SUMMARY.md
**For:** Users wanting complete understanding

**Contains:**
- Overview of what gets updated
- Configuration explanation
- How to get credentials
- High-level code flow
- State mapping
- Usage examples
- Important details
- Troubleshooting guide
- Concurrency explanation

**Best for:** "I want to understand everything"

---

### 🔧 TEAMS_ARCHITECTURE.md
**For:** Developers wanting technical details

**Contains:**
- Visual request/response flows
- Code flow diagrams
- Configuration to code mapping
- HTTP communication details
- State mapping implementation
- Emoji handling
- Error handling flow
- Concurrency model
- Token lifecycle
- Complete request/response cycle

**Best for:** "Show me the internals"

---

### 💻 platform/teams.go
**For:** Developers reading source code

**Contains:**
- TeamsUpdater struct
- NewTeamsUpdater() function
- UpdatePresence() implementation
- ClearPresence() implementation
- API endpoint calls
- Error handling
- Request building

**Best for:** "I want to read the code"

---

## Reading Paths

### Path 1: "Just Get It Working" (15 minutes)

1. Read: **TEAMS_QUICK_REFERENCE.md** (5 min)
   - Understand what you need
   - Follow the quick setup path

2. Reference: **TEAMS_SETUP.md** (10 min)
   - Follow "Getting the Token" section
   - Follow "Getting Your User ID" section
   - Update your config

3. Test: 
   ```bash
   ost set --status "test" --state active
   ```

✅ **Done!** Your Teams integration is working.

---

### Path 2: "I Want Full Understanding" (35 minutes)

1. Read: **TEAMS_SUMMARY.md** (10 min)
   - Understand the complete picture
   - Learn about state mapping
   - See usage examples

2. Read: **TEAMS_SETUP.md** (15 min)
   - Get detailed credentials
   - Understand each step
   - Learn security best practices

3. Read: **TEAMS_ARCHITECTURE.md** (10 min)
   - See how it works internally
   - Understand the code flow
   - Learn about API calls

✅ **Done!** You understand Teams integration completely.

---

### Path 3: "I'm a Developer" (40 minutes)

1. Read: **TEAMS_ARCHITECTURE.md** (15 min)
   - Code flows
   - Request/response cycles
   - Concurrency model

2. Read: **platform/teams.go** (10 min)
   - Implementation details
   - Error handling
   - API calls

3. Read: **TEAMS_SETUP.md** (15 min)
   - API endpoints reference
   - Token lifecycle
   - Troubleshooting

✅ **Done!** You can modify or extend Teams integration.

---

## Common Questions - Where to Find Answers

### "How do I set up Teams?"
→ **TEAMS_SETUP.md** - Complete guide with all phases

### "What token do I need?"
→ **TEAMS_QUICK_REFERENCE.md** - Quick answer
→ **TEAMS_SETUP.md** - Detailed steps to get it

### "What is this user_id thing?"
→ **TEAMS_SUMMARY.md** - Explanation of why you need it
→ **TEAMS_SETUP.md** - How to find it

### "Why am I getting 401 error?"
→ **TEAMS_SUMMARY.md** - Troubleshooting section
→ **TEAMS_QUICK_REFERENCE.md** - Quick fix

### "How does Teams update work internally?"
→ **TEAMS_ARCHITECTURE.md** - Complete technical details
→ **platform/teams.go** - Source code

### "What emoji codes work?"
→ **TEAMS_SUMMARY.md** - Emoji section
→ **TEAMS_SETUP.md** - Link to emoji reference

### "Is my token safe?"
→ **TEAMS_SETUP.md** - Security best practices section
→ **TEAMS_SUMMARY.md** - Configuration security section

### "Can I use my personal Teams account?"
→ **TEAMS_SETUP.md** - Security best practices
→ **TEAMS_SUMMARY.md** - Notes on service accounts

### "What's the difference between token and user_id?"
→ **TEAMS_SUMMARY.md** - Configuration fields explanation
→ **TEAMS_ARCHITECTURE.md** - Configuration structure section

---

## Document Relationship

```
Start Here
    ↓
TEAMS_QUICK_REFERENCE.md (overview)
    ├─ Simple setup? → Use it → Done ✅
    └─ Need details?
        ↓
        TEAMS_SETUP.md (detailed walkthrough)
        ├─ Understand the setup ✅
        └─ Want to know more?
            ↓
            TEAMS_SUMMARY.md (complete overview)
            ├─ Understand concepts ✅
            └─ Want to know internals?
                ↓
                TEAMS_ARCHITECTURE.md (technical deep dive)
                ├─ Code flows ✅
                └─ Need source code?
                    ↓
                    platform/teams.go
                    └─ Implementation details ✅
```

---

## Key Concepts by Document

### Configuration
- **TEAMS_QUICK_REFERENCE.md** - What to put where
- **TEAMS_SETUP.md** - How to get each piece
- **TEAMS_SUMMARY.md** - Why you need each piece
- **TEAMS_ARCHITECTURE.md** - Code mapping

### Getting Credentials
- **TEAMS_QUICK_REFERENCE.md** - Commands
- **TEAMS_SETUP.md** - Complete walkthrough (4 phases)
- **TEAMS_ARCHITECTURE.md** - Token lifecycle

### How It Works
- **TEAMS_SUMMARY.md** - High-level overview
- **TEAMS_ARCHITECTURE.md** - Request/response flows
- **platform/teams.go** - Implementation

### Troubleshooting
- **TEAMS_QUICK_REFERENCE.md** - Quick fixes
- **TEAMS_SETUP.md** - Detailed troubleshooting
- **TEAMS_SUMMARY.md** - Common issues

---

## File Sizes & Scope

| Document | Size | Depth | Audience |
|----------|------|-------|----------|
| TEAMS_QUICK_REFERENCE.md | 6.9 KB | Quick | Users/Operators |
| TEAMS_SETUP.md | 14 KB | Deep | Users/Admins |
| TEAMS_SUMMARY.md | 12 KB | Complete | Everyone |
| TEAMS_ARCHITECTURE.md | 13 KB | Technical | Developers |
| platform/teams.go | 7.5 KB | Code | Developers |

---

## How to Use This Index

### New User?
1. Start with **TEAMS_QUICK_REFERENCE.md**
2. If you need help, go to **TEAMS_SETUP.md**

### Want to Understand Everything?
1. Read **TEAMS_SUMMARY.md**
2. Read **TEAMS_SETUP.md** for detailed steps
3. Read **TEAMS_ARCHITECTURE.md** for technical details

### Developer?
1. Start with **TEAMS_ARCHITECTURE.md**
2. Read **platform/teams.go**
3. Refer to **TEAMS_SETUP.md** for API details

### Troubleshooting?
1. Check **TEAMS_QUICK_REFERENCE.md**
2. Read **TEAMS_SUMMARY.md** troubleshooting section
3. Check **TEAMS_SETUP.md** for detailed help

---

## External Resources Referenced

- [Microsoft Graph API](https://learn.microsoft.com/en-us/graph/api/resources/presence)
- [Azure Portal](https://portal.azure.com)
- [Azure App Registration](https://learn.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app)
- [OAuth 2.0 Client Credentials](https://learn.microsoft.com/en-us/azure/active-directory/develop/v2-oauth2-client-creds-grant-flow)
- [Emoji Cheat Sheet](https://www.webfx.com/tools/emoji-cheat-sheet/)

---

## Document Maintenance

All Teams documentation was generated as part of omniStatus project.

**Generated:** September 26, 2024
**omniStatus Version:** 0.1.0
**Go Version:** 1.21+

---

## Still Have Questions?

1. Check the FAQ in **TEAMS_SETUP.md**
2. Read the troubleshooting sections
3. Review the example configurations
4. Check the inline code comments in **platform/teams.go**

All documents are comprehensive and cover the most common questions and scenarios.
