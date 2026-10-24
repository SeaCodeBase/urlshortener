# Bitly 1:1 Clone — Requirements & Design

Date: 2026-10-24
Repo: github.com/SeaCodeBase/urlshortener (branch `feat/bitly-clone`)
Author: Claude (Fable 5.1), from a logged-in walkthrough of app.bitly.com plus the help center and the Bitly API v4 spec.

## 1. Intent

Turn the existing self-hosted URL shortener (Go/Gin + MariaDB + Redis backend, Next.js 16 + shadcn frontend) into a 1:1 functional and visual replica of the Bitly web app (app.bitly.com), as a single-tenant self-hosted product.

"1:1" means: same information architecture, same page layouts, same labels, same interactions and the same feature set that a Bitly user sees, with every feature fully usable (no plan limits, no upsell).

### Explicit exclusions (decided by the owner)

- No billing, plans, usage limits, "Upgrade" buttons, crown icons, or plan-gated UI. Everything is unlocked.
- No AI features: no Bitly Assist panel, "Create with AI", "Analyze with AI", "Find a domain with AI", "Simplify your workflow" banner, "AI traffic" module, Weekly Insights.
- No "Connect your account" / Integrations marketplace (sidebar item, settings page, home carousel). Webhooks and the API are kept because they are developer features, not third-party integrations.
- No SSO, Account discovery, Mobile deep links (deeplink rules), Shopify, Bitly domain registration ("Search for a domain" purchase flow), 2D barcodes / GS1, digital business card.
- Multi-user teams (invite users, roles) are out of scope. One user = one account (organization). Groups are kept because the UI is organised around them.

## 2. Goals / non-goals / anti-goals

Goals
- A user who knows Bitly can use this app without reading anything.
- Every sidebar item works: Home, Links, QR Codes, Pages, Analytics, Campaigns, Custom domains, Settings.
- Analytics are real (from the existing click pipeline), not sample data.
- Keep the current deployment shape: `docker compose` with backend (redirect :8080, API :8081), frontend :3000, MariaDB, Redis.

Non-goals
- Pixel-perfect font metrics (Bitly uses a proprietary "Bitly Display" font). We use Inter.
- Bitly's mobile apps, browser extension.
- Multi-tenant SaaS concerns (rate limiting per plan, abuse detection).

Anti-goals (would make this a failure even if it "works")
- Leaving any "Upgrade"/"Contact us" dead ends in the UI.
- Analytics modules that render placeholder/sample data.
- Breaking existing links, redirects, passkey login, or the click flusher.

## 3. Visual system (measured from the live app with computed styles, 1440×900)

Exact values captured from app.bitly.com on 2026-10-24 (Links page). Use these verbatim.

- **Font**: Bitly ships a proprietary "Bitly Display" face with the stack `"Bitly Display", system-ui, "Segoe UI", Roboto, Helvetica, Arial, sans-serif`. We cannot ship it; use **Inter** (next/font) with the same fallback stack. Base size 14px, weight 400; UI controls 16px/500; letter-spacing 0.16px on buttons.
- **Colours**
  - Canvas (page background) `#F4F6FA` (rgb 244,246,250)
  - Card / sidebar / top bar background `#FFFFFF`
  - Text primary (navy) `#273144` (rgb 39,49,68); text secondary (meta, "No tags", dates) `#526281` (rgb 82,98,129); body default `#000000` is never used directly
  - Primary blue (buttons, links, short link, "Add tag") `#2A5BD7` (oklch 0.5156 0.198 264.2; rgb 42,91,215); hover `#1F4BBF`
  - Sidebar "Create new" `#022D94` (rgb 2,45,148), shadow `0 2px 8px rgba(0,0,0,.24)`
  - Sidebar active item: background `#EDF2FF` (rgb 237,242,255), text+icon `#0C3EBB` (rgb 12,62,187); inactive item text `#273144`
  - Border (inputs, secondary buttons) `#E1E4EC` (oklch 0.9061 0.0159 266.3)
  - Disabled button: background `#F5F6F9`, text `#A3AAB8`
  - Destructive `#D93025`; success `#1E8E3E`
  - Brand orange (logo only) `#EE6123`
- **Layout**
  - Sidebar: 240px wide, full height, white, padding `16px 12px`, no border (canvas contrast only). "Create new" button 216×40 at y=80, radius 4px, 16px/500 white. Nav items 216×40, radius 4px, 16px/500, icon 20px + 12px gap, 8px vertical spacing; divider before Settings. Collapsed width 64px (icons only). Collapse chevron is a 24px circle button straddling the sidebar edge.
  - Top bar: height 56px, white, padding `8px 16px 8px 24px`; left: group switcher button; right: search icon, help icon, avatar (32px circle, navy background, white initial).
  - Content: padding `0 40px` on the canvas; page title (h1) 32px/600, line-height 40px, letter-spacing 0.32px, colour `#273144`, at 32px from top; primary action button right-aligned on the title row.
  - Toolbar row (search 300px, "Filter by created date", "Add filters") 40px tall; selection row ("0 selected", Export/Hide/Tag text buttons, view radios, "Show: Active") 40px tall.
- **Cards**: white, radius **12px**, padding `16px 16px 24px 32px`, no border, no shadow; card gap 16px. Link card title 20px/600 `#273144` letter-spacing 0.2px; short link 16px/500 `#2A5BD7`; destination 16px/500 `#2A5BD7` with ↳ icon; meta row 14px `#526281`; "Add tag" 14px `#2A5BD7`.
- **Buttons**: height 40px, padding `10px 16px`, radius 4px, 16px/500. Primary: `#2A5BD7` bg, white text. Secondary: white bg, 1px `#E1E4EC` border, `#273144` text. Text/icon buttons: transparent, `#273144` (icon 20px) or `#2A5BD7`; icon-only buttons 28px with 4px padding. Disabled: `#F5F6F9` bg, `#A3AAB8` text.
- **Inputs**: height 40px, 16px/400, padding `1px 12px`, 1px `#E1E4EC` border, radius 4px, text `#273144`, focus ring 2px `#2A5BD7`. Search input has a 20px search icon at left (padding-left 40px).
- **Other**: Toggles are switches (shadcn Switch, checked `#1E8E3E` green like Bitly's screenshots). Dropdown "Show: Active" is a 40px secondary-style button with chevron. Date range picker is a dialog with two-month calendar + preset chips. Toasts: sonner, top-center. Empty state: centred bold title + grey subtitle + link.
- Icons: lucide-react mapped from Bitly's Material icons (content_copy→Copy, share→Share2, edit→Pencil, bar_chart→BarChart2, more_horiz→MoreHorizontal, sell→Tag, calendar_today→Calendar, tune→SlidersHorizontal, subdirectory_arrow_right→CornerDownRight, add_circle→PlusCircle, format_list_bulleted/view_agenda/grid_view→List/Rows3/LayoutGrid).
## 4. Information architecture & routes (frontend)

```
/login, /register                       (existing, restyled to Bitly's auth layout)
/dashboard                               Home
/dashboard/links                         Links list
/dashboard/links/create                  Create a new link
/dashboard/links/[id]                    Link details
/dashboard/links/[id]/edit               Edit link
/dashboard/qr-codes                      QR Codes list
/dashboard/qr-codes/create               Create a new QR Code (2 steps)
/dashboard/qr-codes/[id]                 QR Code details
/dashboard/qr-codes/[id]/edit            Edit QR Code (content)
/dashboard/qr-codes/[id]/customize       Customize design
/dashboard/pages                         Pages list (or empty-state landing)
/dashboard/pages/new                     Claim URL + template
/dashboard/pages/[id]                    Page editor (tabs Build | Design | Track)
/dashboard/analytics                     Analytics overview
/dashboard/analytics/dashboards          All dashboards
/dashboard/analytics/dashboards/[id]     Dashboard
/dashboard/campaigns                     Campaigns list
/dashboard/campaigns/[id]                Campaign detail
/dashboard/custom-domains                Custom domains
/dashboard/settings/profile              Profile
/dashboard/settings/account              Account details
/dashboard/settings/groups               Groups
/dashboard/settings/groups/[id]          Group detail (name, default domain)
/dashboard/settings/webhooks             Webhooks
/dashboard/settings/bulk-upload          Bulk upload
/dashboard/settings/activity-log         Activity log
/dashboard/settings/api                  API access tokens
/m/[slug]                                Public Bitly-style page (link-in-bio)
```

Dashboard routes are group-scoped in Bitly (`/{groupGuid}/links`). We keep flat routes and store the active group in the auth store (`activeGroupId`), sent as `X-Group-ID` header; the backend falls back to the user's default group.

## 5. Features, page by page

### 5.1 Home (`/dashboard`)

Layout (centred column, max 1040px):
1. Heading "What do you want to create?" with a segmented pill toggle **Short link | QR Code**.
2. Quick create card:
   - Short link mode: label "Enter your destination URL", input placeholder "Paste a long URL to shorten it...", row "Domain: {domain ▾}" + "custom back-half" input + "Generate" (random back-half) button, checkbox "Also create a QR Code for this link", primary button "Create your Bitly link" (we label it "Create your link").
   - QR Code mode: same input, checkbox "Also create a link for this QR Code", button "Create your code" → goes to the QR customize step after creation.
   - Pressing Enter submits.
   - Success: modal "Your link is ready!" with the short link in a light-blue box, buttons "View link details" and "Copy link"; for QR: "Your code is ready!" with preview, "View code details", "Download".
3. Two cards side by side:
   - "Today's stats": "ENGAGEMENTS {n}" with a timeframe chips row (Today, Last 7 days, Last 30 days); clicking goes to Analytics.
   - "Hi {name}, get started" checklist with progress %: "Shorten a URL" (→ Create link), "Collect 10 clicks or scans" (progress bar n/10, "Share first link", "View links"), "Create a QR Code", "Create a page". Items auto-check from real data; the card hides once all done (Bitly keeps it; we add a dismiss "×" stored per user).
4. "Recent links" card (last 5) — not on Bitly's current home but was in the earlier version; keep it below the two cards. (Builder's call: keep.)

### 5.2 Links list (`/dashboard/links`)

Header: title "Links" (Bitly: "Bitly Links"), right: primary "Create link".
Toolbar row 1: search input "Search links" (300px), button "Filter by created date" (calendar icon), button "Add filters" (sliders icon). Active filters render as chips below with "Clear all filters".
Toolbar row 2: left: checkbox (select all visible) + "{n} selected" + text buttons "Export", "Hide" (or "Unhide" in Hidden view), "Tag" (disabled until selection; Export always enabled = export current result set as CSV). Right: view toggle (3 icon radios: Compact, Default, Grid) and "Show: Active ▾" (Active | Hidden).

Filters dialog ("Filters"): Tags (multi-select), Link type (All | Links with custom back-halves | Links without custom back-halves), Link expiration (Expired | Expiring | No expiration), Attached QR Code (Links with or without attached QR Codes | With | Without), Campaign (select), Created by (single user: shows the user). Buttons: Clear all filters, Cancel, Apply.

Created-date dialog: two inputs (start/end), month calendar, preset chips, Apply/Cancel.

Link card (Default view):
- checkbox; favicon of destination (Google favicon service `https://www.google.com/s2/favicons?domain=…&sz=32`, fallback globe icon);
- title (bold, links to details), short link (blue, opens in new tab) + copy icon;
- "↳ {destination}";
- footer: "{n} engagements" (bar icon), "{date}", tags chips or "No tags" + "Add tag" (inline tag editor);
- right actions: icon buttons Edit (pencil), Share, View details (bar chart), More (⋯).
- ⋯ menu: View link details, View QR Code / Create QR Code, Add to a page, Add to campaign, Hide link (or Show link), Delete.
Compact view: one line per link (title · short link · engagements · date, actions on hover). Grid view: 2–3 cards per row.
Pagination: infinite scroll (page size 20) with the final sentinel text "You've reached the end of your links".
Bulk: Hide → confirm dialog "Hide {n} links?" / "Hide links"; Tag → dialog with tag multi-select + create new; Export → CSV download (Link, Custom Link, Date created, Title, Destination URL, Engagements, Status).
Delete rules (Bitly): a link can be deleted only if it has never been redirected (destination edited) and has no custom back-half; otherwise the Delete item is disabled with tooltip "This link can't be deleted. You can hide it instead." Deleting also deletes attached QR code. Hidden links keep redirecting.

### 5.3 Create link (`/dashboard/links/create`)

Centred form (max 720px), title "Create a new link", right-top link "Bulk upload" (→ settings bulk upload). Collapsible sections with chevrons:

**Link details**
- Destination URL (required, validated http/https, ≤2048 chars). Enter submits.
- Short link domain (select: default domain + verified custom domains of the group) " / " Back-half (optional) + "Generate" button (random 7-char). Validation: 1–255 chars, `[A-Za-z0-9_-]`, case-sensitive; uniqueness per domain; error "That back-half is already taken".
- Title (optional). If empty, the backend fetches the destination's `<title>` (3 s timeout) and uses it; else falls back to the hostname.
- Tags (multi-select combobox with "Create '{x}'").

**Sharing options**
- Switch "Generate a QR Code": reveals Code color swatches (8 presets + custom hex), Logo upload (PNG, 1:1, ≤5 MB), live preview; "More customizations are available after creating."
- Switch "Add to a Bitly Page" (we label "Add to a page"): Page select + "Button label" input; info "Make sure you publish your page's updates after creating this link".

**Advanced settings**
- Switch "UTM parameters": Source, Medium, Campaign, Term, Content (comboboxes remembering previous values), "+ Add custom URL parameter" (key/value rows), live "UTM tag preview" of the final URL. Parameters are appended to the destination before saving.
- Switch "Link expiration": datetime input "Expiration date"; "After expiration: Visitors will be redirected to the error page" with optional "Redirect to URL" (custom domain's 404 URL if set).
- Switch "Dynamic routing": rule rows (Destination URL; Device = any/mobile/tablet/desktop; OS = any/ios/android; Countries multi-select include/exclude); "Rules apply in order, first match wins; all other traffic goes to the default destination."

Footer sticky bar: "Cancel" | primary "Create your link". On success: copy short link to clipboard, toast "Link created and copied", navigate to details.

### 5.4 Link details (`/dashboard/links/[id]`)

Header: back arrow (to list), favicon, title (h1), right: ⋯ (same menu as card), Edit (pencil), "Share" button (dialog: short link + Copy, share buttons: X, Facebook, LinkedIn, WhatsApp, Email, plus "Copy link").
Two-column top:
- Left card "Details": Short link (+copy), Destination (↳), Tags (chips + "Add tag" inline), Created on (long date with tz), Custom back-half history (if edited; list of previous short links).
- Right: card "QR Code" ("+ Create QR Code" or preview + "Download" + "Customize"), card "Pages" ("+ Add to a page" or list of pages containing it).
Card "Dynamic routing" with "+ Add rules" / rule list.
Section "Link analytics - {destination host}" with controls: date range button ("Last 30 days ▾" → picker), comparison select ("No comparison" | "Previous period"), "Compare to a link" (opens link picker; adds a second series), download CSV icon.
Modules (cards):
- Scorecards row: "Total engagements" (all time), "Engagements ({range})" with ▲/▼ % vs previous period when comparison on, sparkline.
- "Engagements over time" line chart (daily; hourly when range ≤ 2 days), legend "Engagements / Previous period".
- "Locations": tabs Countries | Cities; table Country · Engagements · % · (vs prev.) with pagination 10/page; CSV icon.
- "Referrers": table Referrer · Engagements · %.
- "Devices": bar chart Desktop, Mobile, Tablet, Unknown (current vs previous).
- "Browsers" and "Operating systems" (QR style) tables.
Hidden links show a banner "This link is hidden. Show link".

### 5.5 Edit link (`/dashboard/links/[id]/edit`)

Title "Edit link". Section "Short link": info banner "Editing your short link will create a new, separate short link. The current short link will remain active and continue to point to the same destination." `{short link}` + "Edit back-half" (reveals domain/back-half inputs). Section "Destination URL": value + "Redirect" (reveals input to change destination; counts as a redirect, marks the link non-deletable). "Optional details": Title, Tags. Footer: Cancel | Save.
Semantics: changing the back-half creates a new link (copy title/tags/destination/QR? no QR) and records the old id in `custom_back_half_history` of the new link; the old stays. Changing destination updates in place and invalidates the redirect cache.

### 5.6 QR Codes

List (`/dashboard/qr-codes`): same toolbar as links ("Search codes", Filter by created date, Add filters: Tags, Code expiration, Attached link; Show Active/Hidden; Export). Card: QR thumbnail (120px, rendered from stored customization), title, "Website" type label, "↳ destination", "{n} scans", date, "bit.ly/xxx" short link if coupled, tags; actions: Edit (content), Customize (palette icon), Download (PNG/SVG/JPG dropdown), "View details", ⋯ (View details, Duplicate design, Hide/Show, Delete).

Create (`/dashboard/qr-codes/create`): stepper "Configure code → Customize design"; title "Create a new QR Code"; right column "Preview" (live).
Step 1 "Code details": Destination URL; Domain select; switch "Share as a short link" (back-half + Generate); Title; Tags; "Advanced settings": UTM parameters, Code expiration, Dynamic routing (same components as links). Footer: Cancel | "Design your code →".
Step 2 "Customize design": sections "Select a style" (Pattern: standard, rounded, circle, diamond, classy; Corners: square, rounded, extra-rounded, dot), "Choose your colors" (Code color swatches + hex, Background color + hex, "Reset to default"), "Logo" (upload / remove, size slider), "Frame" (none, border, text bottom, text top; frame text + color), "Advanced": error correction (L/M/Q/H, default H). Footer: "← Back" | Cancel | primary "Create your code". Success modal with download.
Every QR is coupled to a link (Bitly's "coupled" model); the encoded URL is `{short_url}?r=qr`. The redirect records the click with `source=qr` so scans and clicks are separated.
Details page: header like links; "Details" (QR image large, destination, short link, tags, created), "Scan data": Total scans, Scans over time, Scans by location, Scans by browser, Scans by operating system, with the same date/comparison controls. Download button with format menu.
Rendering: frontend uses `qr-code-styling` (supports dots/corners/colors/logo); backend stores `render_customizations` JSON and serves `GET /api/qr-codes/{id}/image?format=svg|png` using `github.com/skip2/go-qrcode` for plain variants (used in exports/emails); the frontend renders the styled version for download.

### 5.7 Pages (link-in-bio)

List: empty state "Pages that turn visits into engagements" + "Create your first page"; otherwise cards (title, URL `{base}/m/{slug}`, views, status Draft/Published, actions Edit, Share, ⋯ Hide/Delete).
New: "Claim your URL": `{domain}/m/` + slug input (3–40 chars, `[a-z0-9-]`, availability check), "Create your page"; then "Start from scratch" or templates (3 simple themes).
Editor (`/dashboard/pages/[id]`): top bar: page title, "Preview", "Publish changes" (disabled when no draft changes), Share, ⋯. Tabs:
- Build: block list with drag reorder (dnd-kit). "+ Add" → Link (title, description, thumbnail URL, existing short link or new destination → creates a link with source page), Text (heading/body), Image (URL), Social icons (platform + URL list). Block toggles: active, pinned. Edit in a side sheet.
- Design: profile image (URL upload to `/api/uploads`), display name, bio; theme presets (5); background (solid/gradient); text color; button style (fill/outline, radius square/rounded/pill); font (Inter/serif/mono).
- Track: Views total, Views over time, Link clicks per block, Views by referrer/device/location, date range.
Public page `/m/[slug]` (Next route, server-rendered): records a view via `POST /api/public/pages/{slug}/view` (ip hash, UA, referrer); each link button is its short link (click = engagement with `source=page`). Redirect server answers `GET /m/:slug` with 302 to `{frontend_url}/m/{slug}`.
Draft vs published: `pages.published_content` JSON snapshot on Publish; editor works on live columns.

### 5.8 Analytics

Overview (`/dashboard/analytics`): title "Analytics". Section "Dashboards": tile "+ Create dashboard", tiles for existing dashboards (name, "Modified on …"), "View all →". Section "Performance overview": "TOTAL ENGAGEMENTS · THIS WEEK" big number, ▲% vs last week, line chart This week vs Last week. Section "Top performing" (top 5 links & QR by engagements, last 30 days).
All dashboards (`/analytics/dashboards`): "Sort by: Date created ▾", "Add filters", "Create new dashboard"; rows: avatar initial, name (link), "Modified on", "Created by", date range, "Showing data for all links and QR Codes" or filter summary; ⋯ Rename, Duplicate, Delete.
Dashboard (`/analytics/dashboards/[id]`): breadcrumb "← Dashboards", editable title (click to rename), "Download CSV" (zip of module CSVs; PDF out of scope), controls: date range (default Last 7 days), comparison select (Previous period | No comparison), "Filter" (dialog: Type Links/QR Codes, select up to 200 items, Apply filters) with summary text "Showing data for all links and QR Codes".
Modules (fixed grid, 2 columns, each with drag handle for reorder, CSV icon, ⋯ → Change visualization):
- "Top day by engagements" scorecard (date, n engagements, ▲% vs prev)
- "Top location by engagements" scorecard (country flag emoji + name)
- "Engagements over time" (line)
- "Engagements by device" (doughnut + legend table)
- "Engagements by referrer" (bar)
- "Engagements by location" (table Country/City with %, vs prev., paginated 10)
- "Links & QR Codes by engagements" (table Type · Link · Engagements · % · vs prev, paginated; export CSV 200/all)
Visualization switch per module: line | bar | table | doughnut | scorecard where sensible. Module order/visualization saved in `dashboards.layout` JSON.
A "Default" dashboard is created for every group automatically.

### 5.9 Campaigns

List: "Campaigns" + "Create campaign"; table/cards: name, channels count, links count, engagements, created; ⋯ Edit, Delete.
Create/Edit dialog: Name, Description, Channels (multi-select of group channels + create new), checkbox "Auto-add UTM parameters" (adds `utm_medium=bitly`, `utm_source={channel}`, `utm_campaign={name}` to links added via this campaign).
Detail: header, tabs "Channels" | "Links"; Channels tab: card per channel with its links and engagements, "Add link to channel" (pick existing or create); Links tab: table of all links with channel column; "Performance" chart: engagements by channel over time (stacked lines) and a summary table.
Adding a link to a campaign from the link ⋯ menu: dialog pick campaign + channel.

### 5.10 Custom domains

Page: title "Custom domains"; if none: hero "Brand your links with a custom domain" with 3 bullet points and primary "Add a domain you already own". List: domain, status chip (Pending | Verified | Error), default badge, groups, added date, ⋯ (Verify now, Set as default for group, Settings, Remove).
Add flow (dialog stepper): 1) enter domain (≤253 chars, subdomain or apex) 2) DNS instructions: CNAME `{domain} → {redirect_host}` (or A record for apex, from config `urls.redirect_host`/`urls.redirect_ip`) and TXT `_urlshortener.{domain} = verify={token}` 3) "Verify" (backend DNS lookup; status + last error) 4) choose group(s) → "Add domain".
Domain settings: Root redirect URL, 404/error redirect URL, HTTPS toggle (informational; TLS termination is outside the app). Unverified domains cannot be selected when creating links. Redirect server resolves `Host` → domain → link (existing behaviour) and applies root/404 redirects.

### 5.11 Settings

Left sub-nav: "Personal settings": Profile; "Account settings": Account details, Groups, Webhooks, Bulk upload, Activity log; "Developer settings": API.
- Profile: Display name + "Update display name"; Email address (read-only + "Change email" dialog requiring password); "Security & authentication": Change password (current/new/confirm, "You will be required to log in after changing your password"); "Passkeys (2-Factor authentication)" — existing passkey list/add/rename/delete, restyled.
- Account details: Account name (editable, defaults to email local part), "Member since", Notification email (read-only).
- Groups: "Using {n} groups", list rows → detail: Group name, Default domain select, "Save"; "Add new group" dialog. Switching active group is in the top bar.
- Webhooks: table (name, URL, event, status Active/Alert/Disabled, last delivery), "Create webhook" dialog (Name, URL, Event: engagement, Group, Secret auto-generated), row actions Verify (sends ping), Edit, Deactivate, Delete; deliveries panel with last 20 attempts (status code, time).
- Bulk upload: upload CSV (columns: long_url, title, custom_back_half, tags, domain) with template download; job rows (file, status, processed/succeeded/failed, created, "Download results CSV"); processed by a background worker in batches of 50.
- Activity log: table (time, actor, action, target, details) with date filter; actions: link.create/update/hide/unhide/delete, qr.*, page.*, campaign.*, domain.*, webhook.*, token.*, bulk.*; 90-day retention.
- API: "Access tokens": "Generate token" (dialog: name, re-enter password → shows token once), list (name, created, last used, Revoke). Tokens are accepted as `Authorization: Bearer` on `/api/*` identically to JWTs.

### 5.12 Global search

Top-bar search: typing shows a popover "Search for \"{q}\" in: [Links] [QR Codes]"; choosing navigates to the list with `?q=`. Enter defaults to Links. Search matches destination, short link, title, tags.

### 5.13 Auth pages

Restyle login/register to Bitly's auth layout: left column form (logo, "Log in and start sharing" / "Create your account", Email, Password, primary button, "Forgot your password?" (hidden if no mail config), link to the other page), right column illustration placeholder with brand colour. Passkey step stays as a second panel. "Continue with Google/Apple/SSO" are not implemented (excluded).

## 6. Backend design

### 6.1 Data model changes (new migration `002_bitly_clone.sql`)

```
groups(id, user_id, name, default_domain_id NULL, created_at, updated_at)
users += default_group_id NULL, account_name NULL, home_checklist_dismissed BOOL
links += group_id NOT NULL, archived BOOL default 0, has_custom_back_half BOOL, redirect_count INT default 0,
         parent_link_id NULL (set on back-half edit), long_url_host VARCHAR(255), source ENUM('link','qr','page') default 'link'
link_tags(link_id, tag VARCHAR(100)) PK(link_id, tag); INDEX(tag)
clicks += source ENUM('link','qr','page') default 'link', os VARCHAR(50)
qr_codes(id, user_id, group_id, link_id UNIQUE, title, render_customizations JSON, archived, expires_at NULL, created_at, updated_at)
qr_tags(qr_code_id, tag)
dynamic_routing_rules(id, link_id, position, long_url, device_match JSON, os_match JSON, country_match JSON, country_exclude JSON)
pages(id, user_id, group_id, slug UNIQUE, title, description, avatar_url, design JSON, status ENUM('draft','published'), published_content JSON NULL, published_at NULL, archived, created_at, updated_at)
page_blocks(id, page_id, type ENUM('link','text','image','social'), position, content JSON, is_active, is_pinned, link_id NULL)
page_views(id, page_id, viewed_at, ip_hash, referrer, country, city, device_type, browser)
campaigns(id, group_id, name, description, auto_utm BOOL, created_at, updated_at)
channels(id, group_id, name, created_at)
campaign_channels(campaign_id, channel_id)
campaign_links(campaign_id, channel_id, link_id) PK(campaign_id, channel_id, link_id)
domains += group_id NULL, verification_token, verification_status ENUM('pending','ok','error') default 'pending', verification_error, verified_at, last_checked_at, root_redirect, not_found_redirect
dashboards(id, group_id, name, date_range VARCHAR(20) default 'last_7_days', filter JSON, layout JSON, created_at, updated_at)
webhooks(id, user_id, group_id, name, url, event VARCHAR(50) default 'engagement', secret, is_active, status ENUM('active','alert','deactivated'), consecutive_failures INT, created_at, updated_at)
webhook_deliveries(id, webhook_id, event_id, status_code, attempt, delivered_at, error)
api_tokens(id, user_id, name, token_hash UNIQUE, token_prefix, last_used_at, created_at, revoked_at)
activity_log(id, user_id, group_id, action VARCHAR(50), entity_type, entity_id, metadata JSON, created_at) INDEX(group_id, created_at)
bulk_uploads(id, user_id, group_id, filename, status ENUM('queued','processing','done','failed'), total, succeeded, failed, result_csv MEDIUMTEXT, created_at, updated_at)
```
Migration backfills: one group per existing user ("{email local part}"), `links.group_id` = that group, `users.default_group_id`. `links.is_active` is kept (expiration/disable); `archived` is Bitly's hide.

### 6.2 Services / endpoints (all under `/api`, JWT or API token)

Group scoping: middleware reads `X-Group-ID` (must belong to user) else default group.

- Groups: `GET/POST /groups`, `GET/PUT /groups/:id`.
- Links: extend `GET /links` with `q, tags[], archived(on|off), custom_back_half(on|off|both), expiration(expired|expiring|none), has_qr(on|off|both), campaign_id, created_after, created_before, sort(created|clicks), page, limit`; response items include `tags[]`, `total_clicks`, `qr_code_id`, `page_ids[]`, `short_url`, `favicon_url`. `POST /links` accepts `tags[]`, `utm{}`, `custom_params[]`, `qr{render_customizations}`, `page_id+button_label`, `routing_rules[]`. `PATCH /links/bulk {action: archive|unarchive|edit_tags, ids[], add_tags[], remove_tags[]}`. `POST /links/:id/back-half {domain_id, back_half}` → new link. `PUT /links/:id` destination change increments `redirect_count`. `DELETE` enforces deletability. `GET /links/export?…` CSV. `GET /links/:id/stats?from&to&unit=day|hour&compare=previous|none` returns current + previous series for all modules. `GET /links/:id/routing-rules`, `PUT`.
- Tags: `GET /tags?type=link|qr`.
- QR: `GET/POST /qr-codes`, `GET/PUT/DELETE /qr-codes/:id`, `PATCH /qr-codes/bulk`, `GET /qr-codes/:id/image?format=svg|png`, `GET /qr-codes/:id/stats` (same shape as link stats but source=qr and adds browsers/os).
- Pages: `GET/POST /pages`, `GET/PUT/DELETE /pages/:id`, `POST /pages/:id/publish`, `GET /pages/check-slug?slug=`, blocks `POST/PUT/DELETE /pages/:id/blocks[/:bid]`, `PUT /pages/:id/blocks/reorder`, `GET /pages/:id/stats`; public `GET /public/pages/:slug` (published content), `POST /public/pages/:slug/view`.
- Analytics: `GET /analytics/overview` (this week vs last week series, top performing); dashboards CRUD `/analytics/dashboards`; `GET /analytics/dashboards/:id/data?from&to&compare` returning every module's data in one response (group-level aggregates over links+qr, honouring the filter); `GET /analytics/dashboards/:id/export`.
- Campaigns: CRUD `/campaigns`, `/channels`; `POST /campaigns/:id/links {channel_id, link_ids[]}`, `DELETE …`; `GET /campaigns/:id/stats`.
- Domains: `GET/POST /domains`, `PUT /domains/:id` (settings, groups), `POST /domains/:id/verify`, `DELETE`.
- Settings: `PUT /auth/me` (display_name, account_name, default_group_id, home_checklist_dismissed), `PUT /auth/email` (password + new email).
- Webhooks: CRUD `/webhooks`, `POST /webhooks/:id/verify`, `GET /webhooks/:id/deliveries`.
- Bulk: `POST /bulk-uploads` (multipart CSV), `GET /bulk-uploads`, `GET /bulk-uploads/:id/results`.
- Activity: `GET /activity-log?from&to&page`.
- Tokens: `GET/POST /api-tokens` (password required), `DELETE /api-tokens/:id`.
- Search: `GET /search?q=` → counts for links and qr codes.
- Uploads: `POST /uploads` (image ≤5 MB → stored under `data/uploads`, served at `/uploads/:name` by the API server).

### 6.3 Redirect server changes

- `GET /:code` → resolve by host+code; if `archived` still redirect; if expired → redirect to domain `not_found_redirect` or render the error page; apply dynamic routing rules (device/os/country from UA + GeoIP) before the default destination; record click with `source` = `qr` when `?r=qr`, `page` when `?r=page`.
- `GET /` on a custom domain → `root_redirect` if set.
- `GET /m/:slug` → 302 to `{urls.frontend_url}/m/{slug}`.
- Error page: minimal HTML "This link has expired" / "Not found" in app style.
- Click event gains `os` (from UA) and `source`.
- Webhook dispatch: after flushing clicks to DB, enqueue webhook events for active webhooks of the link's group (`event_type: engagement`, payload per Bitly: `event_id, datetime, bitlink, long_url, referrer, country, device_type, os, source, tags, group_id`), delivered by a worker with HMAC-SHA256 signature header `X-Signature`; retry policy: 10 s timeout, 5 attempts with backoff; 2 consecutive failures → status `alert`; failing for 24 h → `deactivated`.

### 6.4 Stats queries

Add to ClickRepository: time-bucketed counts (day/hour) by link set + source, totals for a range, breakdowns (country, city, referrer, device, browser, os) for a link set and range, top links by clicks for a group and range. "Previous period" = same length immediately before `from`. Keep `link_stats_daily` updated by the flusher (already exists) and prefer it for day buckets; use `clicks` for hour buckets and breakdowns.

### 6.5 Config additions

```
urls:
  base_url, frontend_url, redirect_host (for CNAME instructions), redirect_ip (for A record)
uploads:
  path: data/uploads
```

## 7. Frontend design

- Keep Next.js app router, Zustand auth store (+ `activeGroupId`), the `/api/[...path]` proxy (add multipart passthrough), TanStack Query for data (already a dependency, start using it), shadcn components: add `switch, select, dropdown-menu, popover, calendar, tabs, badge, tooltip, sheet, command, separator, skeleton, progress, radio-group, textarea, avatar, alert-dialog`.
- New libs: `qr-code-styling`, `@dnd-kit/core` + `@dnd-kit/sortable`, `date-fns`, `react-day-picker` (via shadcn calendar).
- Shared components: `AppShell` (sidebar+topbar), `PageHeader`, `ListToolbar`, `DateRangeDialog`, `FiltersDialog`, `TagPicker`, `ShortLinkField` (domain select + back-half + Generate), `UtmBuilder`, `ExpirationField`, `RoutingRulesEditor`, `QrDesigner`, `QrPreview`, `StatCard`, `TimeSeriesChart`, `BreakdownTable`, `DeviceBarChart`, `DoughnutChart`, `ShareDialog`, `EmptyState`, `ConfirmDialog`.
- Charts: recharts; series colours: current `#2A5BD7`, previous `#A5B8E8` (dashed), doughnut palette `[#2A5BD7,#47B5A1,#F3A33B,#8C6BE0,#E25A5A,#9AA5B1]`.
- Theme tokens in `globals.css` replace the orange primary with Bitly blue; orange kept as `--brand`.

## 8. Phasing (each phase = one implementation plan, built by subagents with TDD, reviewed by Opus 5.5)

1. **Foundation**: migration + groups + tags + archived + sources + new config; AppShell/visual system; auth pages restyle; Home quick create; Links list/create/details/edit (incl. filters, bulk, hide, export, share, back-half edit, UTM, expiration, routing rules); global search; Settings (profile, account, groups, API tokens, activity log).
2. **QR Codes**: model, create 2-step, customize, list, details, scan analytics, link↔QR coupling in link pages, Home QR mode.
3. **Analytics**: link stats v2 (range/compare), overview, dashboards (CRUD, data, filter, export), Home "Today's stats".
4. **Pages**: model, editor (build/design/track), public page, views tracking, link "Add to a page".
5. **Campaigns, Custom domains, Webhooks, Bulk upload**: all four features end-to-end.
6. **Polish**: compact/grid views, keyboard shortcuts (L, Q), empty states, responsive sidebar, error page on redirect server, docs/README, compose + Dockerfiles updated.

## 9. Testing & verification

- Backend: unit tests per service with existing mock pattern (`go.uber.org/mock`), repository tests against MariaDB (`testutil/db.go`), handler tests with httptest. `go vet`, `go test ./...` green.
- Frontend: `npm run lint`, `npx tsc --noEmit`, `npm run build`. Component tests with Vitest + Testing Library for toolbar/filters/forms (add vitest). 
- E2E smoke per phase via the in-app browser against `docker compose` or local dev: register → create link → list → details → edit → hide → QR → page → dashboard.
- Each phase ends with a code review by an Opus 5.5 reviewer agent; findings fixed before the next phase.

## 10. Builder's calls (decisions made here, not by the owner)

- Single user; groups kept; no user invites.
- Dynamic routing included (device/OS/country) because the click pipeline already parses these.
- Pages: 4 block types (link, text, image, social); no YouTube/carousel/grid blocks in v1.
- QR designer limited to patterns/corners/colors/logo/frame; no gradients, no "save design" templates in v1.
- PDF downloads replaced by CSV.
- Favicon via Google's favicon service (config toggle `ui.favicon_service` to disable for offline installs).
- Routes are flat (`/dashboard/...`) with the group in a header, not in the path.

## 11. Decisions after the builder check

1. **In-place upgrades are required.** Production (url.wzq.io) runs on a persistent MariaDB volume, so `docker-entrypoint-initdb.d` will not run new files. The backend gets a minimal migration runner: table `schema_migrations(version VARCHAR(64) PRIMARY KEY, applied_at)`; on startup it applies every `migrations/NNN_*.sql` not yet recorded, in order, each file in one transaction where MariaDB allows it (DDL is not transactional; files must be written idempotently with `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`). `001_init.sql` is recorded as applied when its tables already exist. Existing domains are backfilled with `verification_status='ok'`.
2. **Multi-user stays.** Each registered user is their own account (organization) with one or more groups; `allow_registration` keeps its meaning. All new tables are scoped by `user_id`/`group_id` and every endpoint checks ownership. Page slugs and domains are unique globally. "Created by" filters show the single owner.
3. **QR codes always own a link.** Creating a QR code creates a link row with `source='qr'` (random back-half). If "Share as a short link" is on, the link gets `source='link'` and the QR is attached to it. Links with `source='qr'` or `source='page'` are excluded from the Links list, search, export and checklist; they appear only through their QR code / page. A link's "engagements" = all clicks regardless of source (clicks + scans + page button clicks), matching Bitly; stats endpoints accept `source=` to split them. Deleting a QR code deletes its `source='qr'` link; deleting a link deletes its attached QR.
4. **A domain belongs to at most one group** (`domains.group_id NULL` = usable by all of the user's groups). The add-domain flow offers "All groups" or one group.
5. **Campaign membership is metadata.** Adding an existing link to a channel never rewrites its destination. "Auto-add UTM parameters" only applies when a new link is created from inside a campaign/channel ("Create link" in the channel card) and is applied before shortening. A link may belong to several campaigns/channels.
6. **No stored screenshots.** The reference walkthrough lives in §3 and §5 (labels, layouts, menus) of this document; reviewers check against those texts.
