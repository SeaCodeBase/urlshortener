// Types mirror the backend JSON (snake_case). Timestamps are RFC 3339 strings.

export interface User {
  id: number;
  email: string;
  display_name?: string;
  account_name: string | null;
  default_group_id: number | null;
  home_checklist_dismissed: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface UpdateMeInput {
  display_name?: string;
  account_name?: string;
  default_group_id?: number;
  home_checklist_dismissed?: boolean;
}

export interface UpdateEmailInput {
  password: string;
  new_email: string;
}

// ---------------------------------------------------------------- Groups

export interface Group {
  id: number;
  name: string;
  default_domain_id: number | null;
  created_at: string;
  updated_at: string;
}

export interface GroupsListResponse {
  groups: Group[];
}

export interface UpdateGroupInput {
  name?: string;
  default_domain_id?: number;
  clear_default_domain?: boolean;
}

// ---------------------------------------------------------------- Links

export type LinkSource = "link" | "qr" | "page";

export interface RoutingRule {
  id: number;
  link_id: number;
  position: number;
  long_url: string;
  device_match: string[];
  os_match: string[];
  country_match: string[];
  country_exclude: string[];
}

/** A routing rule as sent by the client (POST /links). */
export interface RoutingRuleInput {
  long_url: string;
  device_match: string[];
  os_match: string[];
  country_match: string[];
  country_exclude: string[];
}

/** GET/PUT /links/:id/routing-rules. */
export interface RoutingRulesResponse {
  routing_rules: RoutingRule[];
}

export interface LinkHistoryEntry {
  id: number;
  short_url: string;
  created_at: string;
}

export interface Link {
  id: number;
  user_id: number;
  group_id: number;
  short_code: string;
  original_url: string;
  title?: string;
  expires_at: string | null;
  is_active: boolean;
  archived: boolean;
  has_custom_back_half: boolean;
  redirect_count: number;
  parent_link_id?: number;
  source: LinkSource;
  domain_id?: number;
  created_at: string;
  updated_at: string;
  short_url: string;
  tags: string[];
  total_clicks: number;
  favicon_url: string;
  /** Present on GET /links/:id (and create/update responses). */
  routing_rules?: RoutingRule[];
  /** Earlier back-halves of this link; present with routing_rules. */
  history?: LinkHistoryEntry[];
  /** The QR code attached to this link, if any. */
  qr_code_id?: number;
  /** Non-archived pages whose blocks reference this link. */
  page_ids?: number[];
}

export interface LinksListResponse {
  links: Link[];
  total: number;
  page: number;
  total_pages: number;
}

export type OnOff = "on" | "off";
export type OnOffBoth = OnOff | "both";

/** Filters of GET /links and GET /links/export (spec §6.2). */
export interface LinkFilters {
  q?: string;
  tags?: string[];
  archived?: OnOffBoth;
  custom_back_half?: OnOffBoth;
  expiration?: "expired" | "expiring" | "none";
  has_qr?: OnOffBoth;
  campaign_id?: number;
  /** YYYY-MM-DD (UTC). */
  created_after?: string;
  /** YYYY-MM-DD (UTC), inclusive. */
  created_before?: string;
  sort?: "created" | "clicks";
}

export interface LinkListParams extends LinkFilters {
  page?: number;
  limit?: number;
}

export interface UTMKeyValue {
  key: string;
  value: string;
}

export interface UTMParams {
  source?: string;
  medium?: string;
  campaign?: string;
  term?: string;
  content?: string;
  custom?: UTMKeyValue[];
}

export interface CreateLinkInput {
  original_url: string;
  custom_code?: string;
  title?: string;
  /** RFC 3339. */
  expires_at?: string;
  domain_id?: number;
  tags?: string[];
  utm?: UTMParams;
  routing_rules?: RoutingRuleInput[];
}

export interface UpdateLinkInput {
  original_url?: string;
  /** "" clears; omitted keeps. */
  title?: string | null;
  /** RFC 3339. */
  expires_at?: string;
  /** Removes the expiration (wins over expires_at). */
  clear_expiration?: boolean;
  is_active?: boolean;
  domain_id?: number;
  /** Replaces the link's tags when present. */
  tags?: string[];
}

export type BulkLinkAction = "archive" | "unarchive" | "edit_tags";

export interface BulkLinksInput {
  action: BulkLinkAction;
  ids: number[];
  add_tags?: string[];
  remove_tags?: string[];
}

export interface BulkLinksResponse {
  updated: number;
}

export interface EditBackHalfInput {
  /** Omitted keeps the link's domain; 0 selects the default domain. */
  domain_id?: number;
  back_half: string;
}

export interface TagsResponse {
  tags: string[];
}

export type TagType = "link" | "qr";

// ---------------------------------------------------------------- QR codes

export type QrPattern = "standard" | "rounded" | "circle" | "diamond" | "classy";
export type QrCorners = "square" | "rounded" | "extra-rounded" | "dot";
export type QrFrame = "none" | "border" | "text-bottom" | "text-top";
export type QrErrorCorrection = "L" | "M" | "Q" | "H";

/** A QR code's design, stored as `render_customizations`. */
export interface QrDesign {
  pattern: QrPattern;
  corners: QrCorners;
  /** #RGB or #RRGGBB. */
  color: string;
  background: string;
  /** Logo image URL (e.g. /uploads/{name}) or null. */
  logo: string | null;
  /** Fraction of the code covered by the logo, 0.2–0.5. */
  logo_size: number;
  frame: QrFrame;
  frame_text: string;
  frame_color: string;
  error_correction: QrErrorCorrection;
}

export const DEFAULT_QR_DESIGN: QrDesign = {
  pattern: "standard",
  corners: "square",
  color: "#000000",
  background: "#FFFFFF",
  logo: null,
  logo_size: 0.3,
  frame: "none",
  frame_text: "Scan me",
  frame_color: "#000000",
  error_correction: "H",
};

export interface QrCode {
  id: number;
  user_id: number;
  group_id: number;
  link_id: number;
  title: string | null;
  render_customizations: Record<string, unknown>;
  archived: boolean;
  created_at: string;
  updated_at: string;
  tags: string[];
  total_scans: number;
  short_url: string;
  qr_url: string;
  original_url: string;
  favicon_url: string;
  expires_at: string | null;
  link_source: "link" | "qr";
  image_url: string;
  /** Present on GET/PUT /qr-codes/:id and create responses (with routing_rules). */
  link?: Link;
}

export interface QrListResponse {
  qr_codes: QrCode[];
  total: number;
  page: number;
  total_pages: number;
}

/** Filters of GET /qr-codes and GET /qr-codes/export. */
export interface QrFilters {
  q?: string;
  tags?: string[];
  archived?: OnOffBoth;
  expiration?: "expired" | "expiring" | "none";
  has_link?: OnOffBoth;
  /** YYYY-MM-DD (UTC). */
  created_after?: string;
  /** YYYY-MM-DD (UTC), inclusive. */
  created_before?: string;
  sort?: "created" | "scans";
}

export interface QrListParams extends QrFilters {
  page?: number;
  limit?: number;
}

export interface CreateQrInput {
  /** Attach to an existing link… */
  link_id?: number;
  /** …or create a new one for this URL. */
  long_url?: string;
  domain_id?: number;
  /** Also list the new link under Links (link_source "link"). */
  share_as_link?: boolean;
  custom_code?: string;
  title?: string;
  tags?: string[];
  utm?: UTMParams;
  /** RFC 3339. */
  expires_at?: string;
  routing_rules?: RoutingRuleInput[];
  render_customizations?: QrDesign;
}

export interface UpdateQrInput {
  title?: string;
  tags?: string[];
  render_customizations?: QrDesign;
  long_url?: string;
  /** RFC 3339. */
  expires_at?: string;
  clear_expiration?: boolean;
  routing_rules?: RoutingRuleInput[];
}

export type BulkQrAction = BulkLinkAction;

export interface BulkQrInput {
  action: BulkQrAction;
  ids: number[];
  add_tags?: string[];
  remove_tags?: string[];
}

/** POST /uploads. */
export interface UploadResponse {
  url: string;
  name: string;
  size: number;
  content_type: string;
}

// ---------------------------------------------------------------- Stats

export type StatsUnit = "day" | "hour";
export type StatsSource = "all" | "link" | "qr" | "page";

export interface LinkStatsQuery {
  /** YYYY-MM-DD (UTC). */
  from?: string;
  /** YYYY-MM-DD (UTC). */
  to?: string;
  unit?: StatsUnit;
  compare?: "previous" | "none";
  source?: StatsSource;
  /** Id of another link of the group whose series is returned as `compare_link`. */
  compare_link?: number;
}

export interface SeriesPoint {
  date: string;
  clicks: number;
}

export interface CountryStat {
  code: string;
  name: string;
  clicks: number;
  percentage: number;
  prev_clicks: number;
}

export interface CityStat {
  name: string;
  country: string;
  clicks: number;
  percentage: number;
  prev_clicks: number;
}

export interface ReferrerStat {
  referrer: string;
  clicks: number;
  percentage: number;
  prev_clicks: number;
}

/** A device, browser or OS breakdown row. */
export interface KeyStat {
  key: string;
  clicks: number;
  percentage: number;
  prev_clicks: number;
}

export interface LinkStatsV2 {
  from: string;
  to: string;
  unit: StatsUnit;
  total_all_time: number;
  total_range: number;
  prev_total_range: number;
  /** null when there is no comparison or the previous total is 0. */
  change_pct: number | null;
  series: SeriesPoint[];
  prev_series: SeriesPoint[];
  countries: CountryStat[];
  cities: CityStat[];
  referrers: ReferrerStat[];
  devices: KeyStat[];
  browsers: KeyStat[];
  os: KeyStat[];
  /** Present when `compare_link` was requested; same window/unit/source. */
  compare_link: CompareLinkStats | null;
}

/** GET /links/:id/stats?compare_link=ID: the other link's series. */
export interface CompareLinkStats {
  id: number;
  title: string;
  short_url: string;
  total_range: number;
  series: SeriesPoint[];
}

/** @deprecated alias kept for older imports; use LinkStatsV2. */
export type LinkStats = LinkStatsV2;

export type TodayRange = "today" | "7d" | "30d";

export interface TodayStatsResponse {
  engagements: number;
  range: TodayRange;
}

// ---------------------------------------------------------------- Analytics dashboards

export type DashboardDateRange = "today" | "last_7_days" | "last_30_days" | "last_60_days" | "last_90_days";

export type DashboardFilter =
  | { type: "all" }
  | { type: "links"; ids: number[] }
  | { type: "qr"; ids: number[] };

export type ModuleKey = "top_day" | "top_location" | "over_time" | "devices" | "referrers" | "locations" | "items";
export type ModuleViz = "scorecard" | "line" | "bar" | "table" | "doughnut";

export interface DashboardModule {
  key: ModuleKey;
  viz: ModuleViz;
}

/** Each of the seven module keys exactly once, in display order. */
export interface DashboardLayout {
  modules: DashboardModule[];
}

export interface Dashboard {
  id: number;
  group_id: number;
  name: string;
  is_default: boolean;
  date_range: DashboardDateRange;
  filter: DashboardFilter;
  layout: DashboardLayout;
  created_by: { id: number; name: string };
  created_at: string;
  updated_at: string;
}

export interface DashboardsListResponse {
  dashboards: Dashboard[];
  total: number;
}

export type DashboardSort = "created" | "modified" | "name";

/** PATCH /analytics/dashboards/:id: only the fields sent change. */
export interface DashboardPatch {
  name?: string;
  date_range?: DashboardDateRange;
  filter?: DashboardFilter;
  layout?: DashboardLayout;
}

export interface DashboardDataQuery {
  /** YYYY-MM-DD (UTC). */
  from?: string;
  /** YYYY-MM-DD (UTC). */
  to?: string;
  compare?: "previous" | "none";
  unit?: StatsUnit;
}

export type DashboardExportQuery = Omit<DashboardDataQuery, "unit">;

/** A link or QR code row of an analytics item list. */
export interface AnalyticsItem {
  type: "link" | "qr";
  /** Link id for `link`, QR code id for `qr`. */
  id: number;
  link_id: number;
  title: string;
  short_url: string;
  /** Destination URL (favicon). */
  original_url: string;
  clicks: number;
  percentage: number;
  prev_clicks: number;
}

export interface TopDayStat {
  date: string;
  count: number;
  prev_count: number;
  change_pct: number | null;
}

export interface TopLocationStat {
  code: string;
  name: string;
  count: number;
}

/** GET /analytics/dashboards/:id/data. */
export interface DashboardData {
  from: string;
  to: string;
  unit: StatsUnit;
  compare: boolean;
  total: number;
  prev_total: number;
  /** null when the previous total is 0 or there is no comparison. */
  change_pct: number | null;
  /** null when total is 0. */
  top_day: TopDayStat | null;
  /** null when there are no countries. */
  top_location: TopLocationStat | null;
  series: SeriesPoint[];
  prev_series: SeriesPoint[];
  devices: KeyStat[];
  referrers: ReferrerStat[];
  countries: CountryStat[];
  cities: CityStat[];
  items: AnalyticsItem[];
}

export interface OverviewWeek {
  from: string;
  to: string;
  total: number;
  series: SeriesPoint[];
}

/** GET /analytics/overview. */
export interface AnalyticsOverview {
  this_week: OverviewWeek;
  last_week: OverviewWeek;
  change_pct: number | null;
  /** Top 5 items over the last 30 days. */
  top_performing: AnalyticsItem[];
}

export interface SearchResponse {
  /** Number of the active group's links matching q. */
  links: number;
  /** Number of matching QR codes. */
  qr_codes: number;
}

// ---------------------------------------------------------------- Pages

export type PageTheme = "classic" | "bold" | "minimal" | "dark" | "sunset";
export type PageBackgroundType = "solid" | "gradient";
export type PageButtonStyle = "fill" | "outline";
export type PageButtonRadius = "square" | "rounded" | "pill";
export type PageFont = "inter" | "serif" | "mono";

/** pages.design: colours are `#RRGGBB`. */
export interface PageDesign {
  theme: PageTheme;
  background: { type: PageBackgroundType; color: string; color2: string };
  text_color: string;
  button: { style: PageButtonStyle; radius: PageButtonRadius; color: string; text_color: string };
  font: PageFont;
}

export type PageStatus = "draft" | "published";
export type BlockType = "link" | "text" | "image" | "social";
export type SocialPlatform = "instagram" | "x" | "facebook" | "tiktok" | "youtube" | "linkedin" | "email" | "website";

export interface LinkBlockContent {
  /** 1–80 chars. */
  title: string;
  /** ≤160 chars. */
  description: string;
  /** http(s) URL or "". */
  thumbnail_url: string;
}

export interface TextBlockContent {
  /** ≤120 chars; heading or body must be non-empty. */
  heading: string;
  /** ≤1000 chars. */
  body: string;
}

export interface ImageBlockContent {
  /** http(s) URL or `/uploads/...`. */
  url: string;
  /** ≤120 chars. */
  alt: string;
}

export interface SocialItem {
  platform: SocialPlatform;
  /** http(s) URL (`mailto:` for email). */
  url: string;
}

export interface SocialBlockContent {
  /** 1–10 items. */
  items: SocialItem[];
}

export interface BlockContentMap {
  link: LinkBlockContent;
  text: TextBlockContent;
  image: ImageBlockContent;
  social: SocialBlockContent;
}

export type BlockContent = BlockContentMap[BlockType];

/** The link a link block points at (null for other types). */
export interface PageBlockLink {
  id: number;
  title: string;
  short_url: string;
  original_url: string;
}

interface PageBlockBase {
  id: number;
  position: number;
  is_active: boolean;
  is_pinned: boolean;
  link_id: number | null;
  link?: PageBlockLink | null;
}

/** A block of GET /pages/:id; discriminated by `type`. */
export type PageBlock = {
  [T in BlockType]: PageBlockBase & { type: T; content: BlockContentMap[T] };
}[BlockType];

/** GET /pages/:id (list items have no `blocks`). */
export interface Page {
  id: number;
  group_id: number;
  slug: string;
  title: string;
  description: string;
  avatar_url: string;
  design: PageDesign;
  status: PageStatus;
  published_at: string | null;
  has_unpublished_changes: boolean;
  archived: boolean;
  /** Shareable `{BaseURL}/m/{slug}`. */
  url: string;
  /** All-time views. */
  views: number;
  created_at: string;
  updated_at: string;
  blocks?: PageBlock[];
}

export interface PagesListResponse {
  pages: Page[];
  total: number;
}

/** GET /pages filters. */
export interface PageFilters {
  /** Title/slug substring. */
  q?: string;
  /** Default off. */
  archived?: OnOff;
}

export type PageTemplate = "scratch" | "classic" | "bold" | "minimal";

export interface CreatePageInput {
  slug: string;
  title?: string;
  template?: PageTemplate;
}

export interface UpdatePageInput {
  title?: string;
  description?: string;
  avatar_url?: string;
  design?: PageDesign;
}

/** POST /pages/:id/blocks: link blocks need `link_id` or `new_link`. */
export type BlockInput = {
  [T in BlockType]: {
    type: T;
    content: BlockContentMap[T];
    link_id?: number;
    new_link?: { original_url: string; title?: string };
  };
}[BlockType];

/** PUT /pages/:id/blocks/:bid. */
export interface BlockPatch {
  content?: BlockContent;
  is_active?: boolean;
  is_pinned?: boolean;
}

export interface ReorderBlocksResponse {
  blocks: PageBlock[];
}

export type SlugCheckReason = "" | "invalid" | "reserved" | "taken";

/** GET /pages/check-slug. */
export interface SlugCheck {
  slug: string;
  available: boolean;
  reason: SlugCheckReason;
}

interface PublishedBlockBase {
  id: number;
  is_pinned: boolean;
}

/** A block of the published snapshot: active only, pinned first; link blocks carry `short_url` (`?r=page`). */
export type PublishedBlock = {
  [T in BlockType]: PublishedBlockBase & {
    type: T;
    content: BlockContentMap[T];
    short_url?: string;
  };
}[BlockType];

/** GET /public/pages/:slug. */
export interface PublishedPage {
  slug: string;
  title: string;
  description: string;
  avatar_url: string;
  design: PageDesign;
  published_at: string;
  blocks: PublishedBlock[];
}

export interface PageStatsQuery {
  /** YYYY-MM-DD (UTC). */
  from?: string;
  /** YYYY-MM-DD (UTC). */
  to?: string;
  unit?: StatsUnit;
  compare?: "previous" | "none";
}

export interface PageBlockStat {
  block_id: number;
  title: string;
  link_id: number | null;
  short_url: string;
  clicks: number;
}

/** GET /pages/:id/stats; `series[].clicks` are views. */
export interface PageStats {
  from: string;
  to: string;
  unit: StatsUnit;
  total_views_all_time: number;
  total_views: number;
  prev_total_views: number;
  /** null when the previous total is 0 or there is no comparison. */
  change_pct: number | null;
  series: SeriesPoint[];
  prev_series: SeriesPoint[];
  referrers: ReferrerStat[];
  devices: KeyStat[];
  countries: CountryStat[];
  cities: CityStat[];
  blocks: PageBlockStat[];
}

// ---------------------------------------------------------------- Domains

export type DomainVerificationStatus = "pending" | "ok" | "error";

export interface Domain {
  id: number;
  user_id: number;
  /** null: usable by all of the user's groups. */
  group_id: number | null;
  domain: string;
  verification_token: string;
  verification_status: DomainVerificationStatus;
  verification_error: string | null;
  verified_at: string | null;
  last_checked_at: string | null;
  root_redirect: string | null;
  not_found_redirect: string | null;
  created_at: string;
}

export interface DomainsListResponse {
  domains: Domain[];
}

export interface CreateDomainInput {
  domain: string;
  group_id?: number | null;
}

export interface UpdateDomainInput {
  /** "" clears it. */
  root_redirect?: string;
  /** "" clears it. */
  not_found_redirect?: string;
  /** null makes the domain usable by all groups. */
  group_id?: number | null;
}

// ---------------------------------------------------------------- API tokens

export interface APIToken {
  id: number;
  name: string;
  prefix: string;
  last_used_at: string | null;
  created_at: string;
}

export interface TokensListResponse {
  tokens: APIToken[];
}

export interface CreateTokenInput {
  name: string;
  password: string;
}

/** POST /api-tokens: the raw token is only returned once. */
export interface CreateTokenResponse {
  token: string;
  id: number;
  name: string;
  prefix: string;
  created_at: string;
}

// ---------------------------------------------------------------- Activity

export interface ActivityEntry {
  id: number;
  user_id: number;
  group_id: number;
  action: string;
  entity_type: string;
  entity_id: number;
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface ActivityParams {
  /** YYYY-MM-DD. */
  from?: string;
  /** YYYY-MM-DD. */
  to?: string;
  page?: number;
  limit?: number;
}

export interface ActivityListResponse {
  entries: ActivityEntry[];
  total: number;
  page: number;
  total_pages: number;
}

// ---------------------------------------------------------------- Meta

export interface Meta {
  base_url: string;
  redirect_host: string;
  redirect_ip: string;
  favicon_service: boolean;
  allow_registration: boolean;
}

// ---------------------------------------------------------------- Passkeys

export interface Passkey {
  id: number;
  user_id: number;
  name: string;
  created_at: string;
  last_used_at?: string;
}
