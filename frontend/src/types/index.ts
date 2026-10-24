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
}

/** @deprecated alias kept for older imports; use LinkStatsV2. */
export type LinkStats = LinkStatsV2;

export type TodayRange = "today" | "7d" | "30d";

export interface TodayStatsResponse {
  engagements: number;
  range: TodayRange;
}

export interface SearchResponse {
  /** Number of the active group's links matching q. */
  links: number;
  /** Number of matching QR codes. */
  qr_codes: number;
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
