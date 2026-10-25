import type {
  ActivityListResponse,
  ActivityParams,
  AnalyticsOverview,
  AuthResponse,
  BulkLinksInput,
  BulkLinksResponse,
  BulkQrInput,
  BulkUpload,
  BulkUploadsListResponse,
  CampaignDetail,
  CampaignInput,
  CampaignsListResponse,
  CampaignStats,
  CampaignStatsQuery,
  Channel,
  ChannelsListResponse,
  CreateLinkInChannelInput,
  CreateQrInput,
  CreateDomainInput,
  CreateLinkInput,
  CreateTokenInput,
  CreateTokenResponse,
  Dashboard,
  DashboardData,
  DashboardDataQuery,
  DashboardExportQuery,
  DashboardPatch,
  DashboardSort,
  DashboardsListResponse,
  Domain,
  DomainsListResponse,
  EditBackHalfInput,
  Group,
  GroupsListResponse,
  Link,
  LinkFilters,
  LinkListParams,
  LinksListResponse,
  LinkStatsQuery,
  LinkStatsV2,
  Meta,
  BlockInput,
  BlockPatch,
  CreatePageInput,
  Page,
  PageBlock,
  PageFilters,
  PagesListResponse,
  PageStats,
  PageStatsQuery,
  Passkey,
  PublishedPage,
  ReorderBlocksResponse,
  SlugCheck,
  UpdatePageInput,
  QrCode,
  QrFilters,
  QrListParams,
  QrListResponse,
  RoutingRuleInput,
  RoutingRulesResponse,
  SearchResponse,
  TagsResponse,
  TagType,
  TodayRange,
  TodayStatsResponse,
  TokensListResponse,
  UpdateDomainInput,
  UpdateEmailInput,
  UpdateGroupInput,
  UpdateLinkInput,
  UpdateMeInput,
  UpdateQrInput,
  UploadResponse,
  User,
  VerifyWebhookResponse,
  Webhook,
  WebhookDeliveriesResponse,
  WebhookInput,
  WebhooksListResponse,
  LinkMembershipsResponse,
} from '@/types';
import type { PublicKeyCredentialCreationOptionsJSON, PublicKeyCredentialRequestOptionsJSON } from '@simplewebauthn/browser';

// API requests go through Next.js API route proxy (/api/[...path])
// The proxy forwards to BACKEND_URL (server-side env var)
const API_BASE = '';

/** An API error response: `message` is the backend's `{error}` text. */
export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

type QueryValue = string | number | boolean | null | undefined | readonly (string | number)[];

/**
 * Builds `?a=1&b=2`, skipping undefined/null/"" values. Arrays repeat the key
 * (`tags=a&tags=b`), which the backend reads with QueryArray.
 */
export function buildQuery(params: object = {}): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params) as [string, QueryValue][]) {
    if (value === undefined || value === null || value === '') continue;
    if (Array.isArray(value)) {
      for (const item of value) search.append(key, String(item));
    } else {
      search.append(key, String(value));
    }
  }
  const qs = search.toString();
  return qs ? `?${qs}` : '';
}

/** Per-request options on top of fetch's RequestInit. */
interface RequestOptions extends RequestInit {
  /**
   * A 401 here means bad user input (e.g. a wrong current password), not an
   * expired session: never sign out for it.
   */
  noAuthLogout?: boolean;
}

/** The auth middleware's 401 messages — the only ones that mean the session is gone. */
const SESSION_401_MESSAGES = new Set([
  'authorization header required',
  'invalid authorization header format',
  'invalid or expired token',
]);

class ApiClient {
  private token: string | null = null;
  private groupId: number | null = null;
  private unauthorizedHandler: (() => void) | null = null;

  setToken(token: string | null) {
    this.token = token;
  }

  /**
   * Active group sent as X-Group-ID. Coupling: the auth store pushes its
   * activeGroupId here via `useAuthStore.subscribe` (see stores/auth.ts), so
   * this must only be called from that subscription (or tests) — setting it
   * elsewhere would be overwritten on the next store change.
   */
  setGroupId(groupId: number | null) {
    this.groupId = groupId;
  }

  /**
   * Called once when a request made with a token gets a 401 (the auth store
   * registers its logout here; it clears the token, so later 401s don't loop).
   */
  onUnauthorized(handler: (() => void) | null) {
    this.unauthorizedHandler = handler;
  }

  /**
   * Default headers; ones given in `extra` win. A FormData body gets no
   * Content-Type so the browser sets the multipart boundary.
   */
  private headers(extra?: HeadersInit, body?: BodyInit | null): Headers {
    const headers = new Headers(extra);
    if (!headers.has('Content-Type') && !(body instanceof FormData)) {
      headers.set('Content-Type', 'application/json');
    }
    if (this.token && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${this.token}`);
    }
    if (this.groupId != null && !headers.has('X-Group-ID')) {
      headers.set('X-Group-ID', String(this.groupId));
    }
    return headers;
  }

  private async fetchOk(endpoint: string, { noAuthLogout = false, ...options }: RequestOptions = {}): Promise<Response> {
    const hadToken = this.token != null;
    let response: Response;
    try {
      response = await fetch(`${API_BASE}${endpoint}`, {
        ...options,
        headers: this.headers(options.headers, options.body),
      });
    } catch {
      throw new ApiError('Network error', 0);
    }
    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      const message = typeof body?.error === 'string' && body.error ? body.error : 'Request failed';
      // Only a session 401 from the auth middleware, for a token that was sent
      // and is still current, signs the user out: a failed login (no token), a
      // wrong password on an authenticated form, or a stale in-flight request doesn't.
      if (
        response.status === 401 &&
        !noAuthLogout &&
        hadToken &&
        this.token != null &&
        SESSION_401_MESSAGES.has(message)
      ) {
        this.unauthorizedHandler?.();
      }
      throw new ApiError(message, response.status);
    }
    return response;
  }

  private async request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
    const response = await this.fetchOk(endpoint, options);
    if (response.status === 204) {
      return {} as T;
    }
    return response.json();
  }

  private send<T>(method: string, endpoint: string, body?: unknown, opts: Pick<RequestOptions, 'noAuthLogout'> = {}): Promise<T> {
    return this.request<T>(endpoint, {
      ...opts,
      method,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  }

  // Meta (public)
  async getMeta() {
    return this.request<Meta>('/api/meta');
  }

  // Auth
  async register(email: string, password: string) {
    return this.send<AuthResponse>('POST', '/api/auth/register', { email, password });
  }

  async login(email: string, password: string) {
    return this.send<AuthResponse>('POST', '/api/auth/login', { email, password });
  }

  async me() {
    return this.request<User>('/api/auth/me');
  }

  async updateMe(data: UpdateMeInput) {
    return this.send<User>('PUT', '/api/auth/me', data);
  }

  /** @deprecated use updateMe. */
  async updateProfile(displayName: string): Promise<User> {
    return this.updateMe({ display_name: displayName });
  }

  async updateEmail(data: UpdateEmailInput) {
    return this.send<User>('PUT', '/api/auth/email', data, { noAuthLogout: true });
  }

  async changePassword(oldPassword: string, newPassword: string) {
    return this.send<{ message: string }>('PUT', '/api/auth/password', {
      old_password: oldPassword,
      new_password: newPassword,
    }, { noAuthLogout: true });
  }

  // Links
  async getLinks(params: LinkListParams = {}) {
    return this.request<LinksListResponse>(`/api/links${buildQuery(params)}`);
  }

  async createLink(data: CreateLinkInput) {
    return this.send<Link>('POST', '/api/links', data);
  }

  async getLink(id: number) {
    return this.request<Link>(`/api/links/${id}`);
  }

  async updateLink(id: number, data: UpdateLinkInput) {
    return this.send<Link>('PUT', `/api/links/${id}`, data);
  }

  /** Rejects with ApiError status 409 when the link can't be deleted. */
  async deleteLink(id: number) {
    return this.send<void>('DELETE', `/api/links/${id}`);
  }

  async bulkLinks(data: BulkLinksInput) {
    return this.send<BulkLinksResponse>('PATCH', '/api/links/bulk', data);
  }

  /** Creates a new link with the back-half (the old one stays) → 201. */
  async editBackHalf(id: number, data: EditBackHalfInput) {
    return this.send<Link>('POST', `/api/links/${id}/back-half`, data);
  }

  async setLinkTags(id: number, tags: string[]) {
    return this.send<TagsResponse>('PUT', `/api/links/${id}/tags`, { tags });
  }

  /** The filtered links as a CSV file. */
  async exportLinks(params: LinkFilters = {}): Promise<Blob> {
    const response = await this.fetchOk(`/api/links/export${buildQuery(params)}`);
    return response.blob();
  }

  async getRoutingRules(id: number) {
    return this.request<RoutingRulesResponse>(`/api/links/${id}/routing-rules`);
  }

  /** Replaces the link's routing rules (order = priority). */
  async setRoutingRules(id: number, rules: RoutingRuleInput[]) {
    return this.send<RoutingRulesResponse>('PUT', `/api/links/${id}/routing-rules`, { routing_rules: rules });
  }

  async getLinkStats(id: number, query: LinkStatsQuery = {}) {
    return this.request<LinkStatsV2>(`/api/links/${id}/stats${buildQuery(query)}`);
  }

  // QR codes
  async getQrCodes(params: QrListParams = {}) {
    return this.request<QrListResponse>(`/api/qr-codes${buildQuery(params)}`);
  }

  async createQrCode(data: CreateQrInput) {
    return this.send<QrCode>('POST', '/api/qr-codes', data);
  }

  async getQrCode(id: number) {
    return this.request<QrCode>(`/api/qr-codes/${id}`);
  }

  async updateQrCode(id: number, data: UpdateQrInput) {
    return this.send<QrCode>('PUT', `/api/qr-codes/${id}`, data);
  }

  async deleteQrCode(id: number) {
    return this.send<void>('DELETE', `/api/qr-codes/${id}`);
  }

  async bulkQrCodes(data: BulkQrInput) {
    return this.send<BulkLinksResponse>('PATCH', '/api/qr-codes/bulk', data);
  }

  async setQrTags(id: number, tags: string[]) {
    return this.send<TagsResponse>('PUT', `/api/qr-codes/${id}/tags`, { tags });
  }

  /** The filtered QR codes as a CSV file. */
  async exportQrCodes(params: QrFilters = {}): Promise<Blob> {
    const response = await this.fetchOk(`/api/qr-codes/export${buildQuery(params)}`);
    return response.blob();
  }

  async getQrStats(id: number, query: LinkStatsQuery = {}) {
    return this.request<LinkStatsV2>(`/api/qr-codes/${id}/stats${buildQuery(query)}`);
  }

  /** Uploads an image (multipart field `file`) → 201; 413/400 on bad files. */
  async uploadImage(file: File) {
    const body = new FormData();
    body.append('file', file);
    return this.request<UploadResponse>('/api/uploads', { method: 'POST', body });
  }

  // Group-level analytics, tags, search, activity
  async getTodayStats(range: TodayRange = 'today') {
    return this.request<TodayStatsResponse>(`/api/analytics/today${buildQuery({ range })}`);
  }

  /** This week vs last week (rolling 7 days) and the top 5 items of the last 30 days. */
  async getAnalyticsOverview() {
    return this.request<AnalyticsOverview>('/api/analytics/overview');
  }

  /** The group's dashboards; the first call creates the "Default" one. */
  async listDashboards(sort: DashboardSort = 'created') {
    return this.request<DashboardsListResponse>(`/api/analytics/dashboards${buildQuery({ sort })}`);
  }

  async getDashboard(id: number) {
    return this.request<Dashboard>(`/api/analytics/dashboards/${id}`);
  }

  async createDashboard(name: string) {
    return this.send<Dashboard>('POST', '/api/analytics/dashboards', { name });
  }

  /** 400 on an invalid filter/layout. */
  async updateDashboard(id: number, patch: DashboardPatch) {
    return this.send<Dashboard>('PATCH', `/api/analytics/dashboards/${id}`, patch);
  }

  async duplicateDashboard(id: number) {
    return this.send<Dashboard>('POST', `/api/analytics/dashboards/${id}/duplicate`);
  }

  /** Rejects with ApiError status 409 for the Default dashboard. */
  async deleteDashboard(id: number) {
    return this.send<void>('DELETE', `/api/analytics/dashboards/${id}`);
  }

  async getDashboardData(id: number, query: DashboardDataQuery = {}) {
    return this.request<DashboardData>(`/api/analytics/dashboards/${id}/data${buildQuery(query)}`);
  }

  /** The seven module CSVs as a zip file. */
  async exportDashboard(id: number, query: DashboardExportQuery = {}): Promise<Blob> {
    const response = await this.fetchOk(`/api/analytics/dashboards/${id}/export${buildQuery(query)}`);
    return response.blob();
  }

  // Pages (link-in-bio)
  async listPages(filters: PageFilters = {}) {
    return this.request<PagesListResponse>(`/api/pages${buildQuery(filters)}`);
  }

  async getPage(id: number) {
    return this.request<Page>(`/api/pages/${id}`);
  }

  /** 409 when the slug is taken, 400 when invalid/reserved. */
  async createPage(input: CreatePageInput) {
    return this.send<Page>('POST', '/api/pages', input);
  }

  async updatePage(id: number, patch: UpdatePageInput) {
    return this.send<Page>('PUT', `/api/pages/${id}`, patch);
  }

  async setPageArchived(id: number, archived: boolean) {
    return this.send<Page>('PATCH', `/api/pages/${id}`, { archived });
  }

  async deletePage(id: number) {
    return this.send<void>('DELETE', `/api/pages/${id}`);
  }

  /** Snapshots the current content as the public page. */
  async publishPage(id: number) {
    return this.send<Page>('POST', `/api/pages/${id}/publish`);
  }

  async checkSlug(slug: string) {
    return this.request<SlugCheck>(`/api/pages/check-slug${buildQuery({ slug })}`);
  }

  async addBlock(pageId: number, input: BlockInput) {
    return this.send<PageBlock>('POST', `/api/pages/${pageId}/blocks`, input);
  }

  async updateBlock(pageId: number, blockId: number, patch: BlockPatch) {
    return this.send<PageBlock>('PUT', `/api/pages/${pageId}/blocks/${blockId}`, patch);
  }

  async deleteBlock(pageId: number, blockId: number) {
    return this.send<void>('DELETE', `/api/pages/${pageId}/blocks/${blockId}`);
  }

  /** `ids` must be exactly the page's block ids in the new order (else 400). */
  async reorderBlocks(pageId: number, ids: number[]) {
    return this.send<ReorderBlocksResponse>('PUT', `/api/pages/${pageId}/blocks/reorder`, { ids });
  }

  async getPageStats(id: number, query: PageStatsQuery = {}) {
    return this.request<PageStats>(`/api/pages/${id}/stats${buildQuery(query)}`);
  }

  /** Published page (no auth); 404 for drafts and unknown slugs. The /m/[slug] server route fetches the backend directly. */
  async getPublicPage(slug: string) {
    return this.request<PublishedPage>(`/api/public/pages/${encodeURIComponent(slug)}`);
  }

  /** View beacon → 204 (404 when unpublished). `referrer` is the visitor's document.referrer. */
  async recordPageView(slug: string, referrer?: string) {
    return this.send<void>(
      'POST',
      `/api/public/pages/${encodeURIComponent(slug)}/view`,
      referrer === undefined ? undefined : { referrer },
    );
  }

  async getTags(type: TagType = 'link') {
    return this.request<TagsResponse>(`/api/tags${buildQuery({ type })}`);
  }

  async search(q: string) {
    return this.request<SearchResponse>(`/api/search${buildQuery({ q })}`);
  }

  async getActivity(params: ActivityParams = {}) {
    return this.request<ActivityListResponse>(`/api/activity-log${buildQuery(params)}`);
  }

  // Campaigns
  async listCampaigns() {
    return this.request<CampaignsListResponse>('/api/campaigns');
  }

  async getCampaign(id: number) {
    return this.request<CampaignDetail>(`/api/campaigns/${id}`);
  }

  /** 400 on an invalid name/description. */
  async createCampaign(input: CampaignInput) {
    return this.send<CampaignDetail>('POST', '/api/campaigns', input);
  }

  async updateCampaign(id: number, input: CampaignInput) {
    return this.send<CampaignDetail>('PUT', `/api/campaigns/${id}`, input);
  }

  async deleteCampaign(id: number) {
    return this.send<void>('DELETE', `/api/campaigns/${id}`);
  }

  /** Adds existing links (source "link", same group) to a channel of the campaign → 204. */
  async addCampaignLinks(campaignId: number, channelId: number, linkIds: number[]) {
    return this.send<void>('POST', `/api/campaigns/${campaignId}/links`, { channel_id: channelId, link_ids: linkIds });
  }

  async removeCampaignLink(campaignId: number, channelId: number, linkId: number) {
    return this.send<void>('DELETE', `/api/campaigns/${campaignId}/links`, { channel_id: channelId, link_id: linkId });
  }

  /** Creates a link (UTM-tagged when the campaign has auto_utm) and adds it to the channel → 201. */
  async createLinkInChannel(campaignId: number, channelId: number, input: CreateLinkInChannelInput) {
    return this.send<Link>('POST', `/api/campaigns/${campaignId}/channels/${channelId}/links`, input);
  }

  async getCampaignStats(id: number, query: CampaignStatsQuery = {}) {
    return this.request<CampaignStats>(`/api/campaigns/${id}/stats${buildQuery(query)}`);
  }

  async getLinkCampaigns(linkId: number) {
    return this.request<LinkMembershipsResponse>(`/api/links/${linkId}/campaigns`);
  }

  async listChannels() {
    return this.request<ChannelsListResponse>('/api/channels');
  }

  /** 409 when the group already has a channel of that name. */
  async createChannel(name: string) {
    return this.send<Channel>('POST', '/api/channels', { name });
  }

  // Webhooks
  async listWebhooks() {
    return this.request<WebhooksListResponse>('/api/webhooks');
  }

  async getWebhook(id: number) {
    return this.request<Webhook>(`/api/webhooks/${id}`);
  }

  /** 400 when the URL is not a public http(s) address. */
  async createWebhook(input: WebhookInput) {
    return this.send<Webhook>('POST', '/api/webhooks', input);
  }

  async updateWebhook(id: number, input: WebhookInput) {
    return this.send<Webhook>('PUT', `/api/webhooks/${id}`, input);
  }

  async deleteWebhook(id: number) {
    return this.send<void>('DELETE', `/api/webhooks/${id}`);
  }

  /** Sends a signed ping; returns the recorded delivery and the updated webhook. */
  async verifyWebhook(id: number) {
    return this.send<VerifyWebhookResponse>('POST', `/api/webhooks/${id}/verify`);
  }

  async deactivateWebhook(id: number) {
    return this.send<Webhook>('POST', `/api/webhooks/${id}/deactivate`);
  }

  async activateWebhook(id: number) {
    return this.send<Webhook>('POST', `/api/webhooks/${id}/activate`);
  }

  /** The last 20 deliveries, newest first. */
  async getWebhookDeliveries(id: number) {
    return this.request<WebhookDeliveriesResponse>(`/api/webhooks/${id}/deliveries`);
  }

  // Bulk uploads
  /** Queues a CSV (multipart field `file`) → 201; 400 on a bad header, > 1000 rows or > 2 MiB. */
  async uploadBulkCsv(file: File) {
    const body = new FormData();
    body.append('file', file);
    return this.request<BulkUpload>('/api/bulk-uploads', { method: 'POST', body });
  }

  async listBulkUploads() {
    return this.request<BulkUploadsListResponse>('/api/bulk-uploads');
  }

  async getBulkUpload(id: number) {
    return this.request<BulkUpload>(`/api/bulk-uploads/${id}`);
  }

  /** The job's result CSV; rejects with ApiError status 409 until the job is done or failed. */
  async downloadBulkResults(id: number): Promise<Blob> {
    const response = await this.fetchOk(`/api/bulk-uploads/${id}/results`);
    return response.blob();
  }

  // Groups
  async getGroups() {
    return this.request<GroupsListResponse>('/api/groups');
  }

  async getGroup(id: number) {
    return this.request<Group>(`/api/groups/${id}`);
  }

  async createGroup(name: string) {
    return this.send<Group>('POST', '/api/groups', { name });
  }

  async updateGroup(id: number, data: UpdateGroupInput) {
    return this.send<Group>('PUT', `/api/groups/${id}`, data);
  }

  // Domains
  async getDomains() {
    return this.request<DomainsListResponse>('/api/domains');
  }

  async createDomain(data: string | CreateDomainInput) {
    const body = typeof data === 'string' ? { domain: data } : data;
    return this.send<Domain>('POST', '/api/domains', body);
  }

  async updateDomain(id: number, data: UpdateDomainInput) {
    return this.send<Domain>('PUT', `/api/domains/${id}`, data);
  }

  async verifyDomain(id: number) {
    return this.send<Domain>('POST', `/api/domains/${id}/verify`);
  }

  async deleteDomain(id: number) {
    return this.send<{ message: string }>('DELETE', `/api/domains/${id}`);
  }

  // API tokens
  async getTokens() {
    return this.request<TokensListResponse>('/api/api-tokens');
  }

  async createToken(data: CreateTokenInput) {
    return this.send<CreateTokenResponse>('POST', '/api/api-tokens', data, { noAuthLogout: true });
  }

  async revokeToken(id: number) {
    return this.send<void>('DELETE', `/api/api-tokens/${id}`);
  }

  // Passkeys
  async getPasskeys(): Promise<Passkey[]> {
    return this.request('/api/auth/passkeys');
  }

  async beginPasskeyRegistration(): Promise<{ options: { publicKey: PublicKeyCredentialCreationOptionsJSON }; session_data: string }> {
    return this.send('POST', '/api/auth/passkeys/register/begin');
  }

  async renamePasskey(id: number, name: string): Promise<void> {
    return this.send('PUT', `/api/auth/passkeys/${id}`, { name });
  }

  async deletePasskey(id: number): Promise<void> {
    return this.send('DELETE', `/api/auth/passkeys/${id}`);
  }

  async beginPasskeyVerify(userId: number): Promise<{ options: { publicKey: PublicKeyCredentialRequestOptionsJSON }; session_data: string }> {
    return this.send('POST', '/api/auth/passkeys/verify/begin', { user_id: userId }, { noAuthLogout: true });
  }
}

export const api = new ApiClient();
