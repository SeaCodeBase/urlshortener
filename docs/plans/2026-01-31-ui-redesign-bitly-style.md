# UI Redesign - Bitly Style

## Overview

Redesign the URL Shortener dashboard to adopt Bitly's visual style, including a sidebar navigation, improved user menu, and enhanced features like Display Name and Passkey 2FA.

## Design Goals

1. Adopt Bitly-style sidebar navigation layout
2. Add user Display Name feature
3. Implement Passkey-based 2FA
4. Create dedicated "Create Link" page with card-based form
5. Apply orange color theme

---

## 1. Layout Structure

### Overall Layout (Three-column)

```
┌─────────────────────────────────────────────────────────┐
│ [Logo]              [Search...]        [?] [Avatar ▼]   │  ← Top Navigation
├────────────┬────────────────────────────────────────────┤
│            │                                            │
│ [Create    │                                            │
│   new]     │         Main Content Area                  │
│            │                                            │
│ ○ Home     │                                            │
│ ● Links    │                                            │
│ ○ Analytics│                                            │
│ ○ Settings │                                            │
│            │                                            │
└────────────┴────────────────────────────────────────────┘
```

### Sidebar (~200px fixed width)

- **Top**: Logo + collapse toggle button
- **"Create new" button**: Primary orange button
- **Navigation menu**:
  - Home (Dashboard overview)
  - Links (Link list and management)
  - Analytics (Stats and charts)
  - Settings (User preferences, Passkey management)
- Each item has an icon, selected item has gray background highlight with left orange border

### Top Navigation Bar

- **Left**: Empty or breadcrumb
- **Right**: Search box, help icon, user avatar dropdown

---

## 2. User Menu & Display Name

### User Avatar Dropdown Menu

```
┌──────────────────────┐
│  [J]  Jose           │  ← Avatar (initial) + Display name
│       jose@email.com │  ← Email
├──────────────────────┤
│  User ID: 12345      │  ← User ID (copyable)
├──────────────────────┤
│  Sign out            │
└──────────────────────┘
```

### User Model Changes

```go
type User struct {
    ID           uint64    `db:"id" json:"id"`
    Email        string    `db:"email" json:"email"`
    DisplayName  string    `db:"display_name" json:"display_name"`  // NEW
    PasswordHash string    `db:"password_hash" json:"-"`
    CreatedAt    time.Time `db:"created_at" json:"created_at"`
    UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
```

### Display Name Behavior

- Optional during registration (defaults to email prefix before @)
- Editable in Settings page
- Displayed in top navigation; falls back to email if empty

### API Changes

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/register` | Add optional `display_name` parameter |
| PUT | `/users/me` | New endpoint to update display_name |
| GET | `/users/me` | Returns display_name in response |

---

## 3. Passkey 2FA

### Data Model

```go
type Passkey struct {
    ID           uint64    `db:"id" json:"id"`
    UserID       uint64    `db:"user_id" json:"user_id"`
    Name         string    `db:"name" json:"name"`              // Custom name, e.g., "MacBook Pro"
    CredentialID []byte    `db:"credential_id" json:"-"`        // WebAuthn credential ID
    PublicKey    []byte    `db:"public_key" json:"-"`           // WebAuthn public key
    Counter      uint32    `db:"counter" json:"-"`              // Signature counter (anti-replay)
    CreatedAt    time.Time `db:"created_at" json:"created_at"`
    LastUsedAt   time.Time `db:"last_used_at" json:"last_used_at"`
}
```

### Database Schema

```sql
CREATE TABLE passkeys (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    credential_id VARBINARY(1024) NOT NULL,
    public_key VARBINARY(1024) NOT NULL,
    counter INT UNSIGNED NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY unique_credential (credential_id)
);
```

### Login Flow

```
User enters email/password
        ↓
  Validate password
        ↓
  Does user have Passkey(s)?
    /          \
   No          Yes
   ↓            ↓
Login OK    Require Passkey verification
                ↓
         Verify success → Login OK
```

### Settings Page - Passkey Management

- List of registered Passkeys (name, created date, last used date)
- "Add Passkey" button → Triggers WebAuthn registration
- Each Passkey can be renamed or deleted
- At least show warning when deleting last Passkey

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/passkeys/register/begin` | Start Passkey registration (returns WebAuthn options) |
| POST | `/auth/passkeys/register/finish` | Complete registration with attestation |
| POST | `/auth/passkeys/verify/begin` | Start verification during login |
| POST | `/auth/passkeys/verify/finish` | Complete verification with assertion |
| GET | `/auth/passkeys` | List user's Passkeys |
| PUT | `/auth/passkeys/:id` | Rename a Passkey |
| DELETE | `/auth/passkeys/:id` | Delete a Passkey |

### Login API Changes

Current login returns token immediately. New flow:

1. `POST /auth/login` with email/password
   - If no Passkey: returns `{ token, user }`
   - If has Passkey: returns `{ requires_passkey: true, challenge: "...", user_id: 123 }`

2. If `requires_passkey`, frontend calls `/auth/passkeys/verify/begin` then `/auth/passkeys/verify/finish`
   - On success: returns `{ token, user }`

---

## 4. Create Link Page

### Route

`/dashboard/links/create`

### Layout (Bitly-style card sections)

```
┌─────────────────────────────────────────────────────────┐
│                    Create a new link                    │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────┐   │
│  │ Link details                              [▼]   │   │
│  ├─────────────────────────────────────────────────┤   │
│  │ Destination URL                                 │   │
│  │ [https://example.com/my-long-url           ]   │   │
│  │                                                 │   │
│  │ Short link domain    /    Back-half (optional)  │   │
│  │ [localhost:8080 ▼]        [my-custom-code    ]  │   │
│  │                                                 │   │
│  │ Title (optional)                                │   │
│  │ [My Link Title                             ]   │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Advanced settings                         [▼]   │   │
│  ├─────────────────────────────────────────────────┤   │
│  │ Expiration date (optional)                      │   │
│  │ [2026-02-28T23:59                          ]   │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
├─────────────────────────────────────────────────────────┤
│           [Cancel]                [Create your link]   │  ← Fixed bottom
└─────────────────────────────────────────────────────────┘
```

### Interaction Details

- Each card section is collapsible/expandable
- "Cancel" navigates back to Links list
- "Create your link" submits form, on success redirects to Links list with success toast
- Form validation shows inline errors

### Dashboard Home Page Changes

- Remove the "Create Short Link" card
- Keep only the "Your Links" table
- Add empty state: "No links yet. Create your first link!" with CTA button

---

## 5. Color Theme & Visual Style

### Primary Colors

```css
:root {
  --primary: #EE6123;           /* Orange - primary buttons, accents */
  --primary-hover: #D55520;     /* Orange hover state */
  --primary-light: #FFF4EF;     /* Light orange - background highlight */
  --primary-foreground: #FFFFFF; /* White text on primary */
}
```

### Sidebar Styles

```css
.sidebar {
  background: #FFFFFF;
  border-right: 1px solid #E5E7EB;
  width: 200px;
}

.sidebar-nav-item {
  color: #374151;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.sidebar-nav-item:hover {
  background: #F3F4F6;
}

.sidebar-nav-item.active {
  background: #F3F4F6;
  border-left: 3px solid #EE6123;
}

.sidebar-create-btn {
  background: #EE6123;
  color: white;
  border-radius: 6px;
  padding: 10px 16px;
  width: 100%;
  margin: 16px;
}
```

### Top Navigation Styles

```css
.top-nav {
  background: #FFFFFF;
  border-bottom: 1px solid #E5E7EB;
  height: 56px;
  padding: 0 24px;
}

.search-box {
  border: 1px solid #D1D5DB;
  border-radius: 6px;
  padding: 8px 12px;
  width: 300px;
}

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #EE6123;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
}
```

### Button Styles

```css
.btn-primary {
  background: #EE6123;
  color: white;
  border: none;
}

.btn-primary:hover {
  background: #D55520;
}

.btn-secondary {
  background: white;
  color: #374151;
  border: 1px solid #D1D5DB;
}

.btn-danger {
  background: #DC2626;
  color: white;
}
```

---

## 6. Implementation Scope

### Frontend Changes

| Priority | Task |
|----------|------|
| 1 | Create Sidebar component with navigation |
| 2 | Create new DashboardLayout with sidebar + top nav |
| 3 | Create UserMenu dropdown component |
| 4 | Create `/dashboard/links/create` page |
| 5 | Update `/dashboard` to show only links list |
| 6 | Update Settings page (Display Name edit, Passkey management) |
| 7 | Update login flow (Passkey verification step) |
| 8 | Apply orange color theme globally |

### Backend Changes

| Priority | Task |
|----------|------|
| 1 | Add `display_name` column to users table |
| 2 | Update User model and related handlers |
| 3 | Create `passkeys` table |
| 4 | Implement Passkey model and repository |
| 5 | Implement WebAuthn registration endpoints |
| 6 | Implement WebAuthn verification endpoints |
| 7 | Update login handler for 2FA flow |
| 8 | Add Passkey management endpoints (list, rename, delete) |

### Database Migrations

```sql
-- Migration 1: Add display_name to users
ALTER TABLE users ADD COLUMN display_name VARCHAR(255) DEFAULT NULL;

-- Migration 2: Create passkeys table
CREATE TABLE passkeys (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    credential_id VARBINARY(1024) NOT NULL,
    public_key VARBINARY(1024) NOT NULL,
    counter INT UNSIGNED NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE KEY unique_credential (credential_id)
);
```

---

## 7. Dependencies

### Backend Libraries

- **go-webauthn/webauthn**: WebAuthn server implementation for Go
  - `go get github.com/go-webauthn/webauthn`

### Frontend Libraries

- **@simplewebauthn/browser**: WebAuthn client-side helpers
  - `npm install @simplewebauthn/browser`

---

## 8. Open Questions

1. **Search functionality**: Should the search box in the top nav search links only, or include other content?
2. **Analytics page**: What metrics should be displayed? (This can be a separate design document)
3. **Passkey backup**: Should we provide backup codes in case user loses all Passkey devices?

---

## Appendix: Screenshots Reference

Bitly Create Link page was used as the primary visual reference for:
- Sidebar navigation structure
- Card-based form layout
- User avatar dropdown menu
- Color scheme (adapted orange)
