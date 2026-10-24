import type {
  ActivityListResponse,
  ActivityParams,
  AuthResponse,
  BulkLinksInput,
  BulkLinksResponse,
  CreateDomainInput,
  CreateLinkInput,
  CreateTokenInput,
  CreateTokenResponse,
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
  Passkey,
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
  User,
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

  /** Default headers; ones given in `extra` win. */
  private headers(extra?: HeadersInit): Headers {
    const headers = new Headers(extra);
    if (!headers.has('Content-Type')) {
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
        headers: this.headers(options.headers),
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

  async getLinkStats(id: number, query: LinkStatsQuery = {}) {
    return this.request<LinkStatsV2>(`/api/links/${id}/stats${buildQuery(query)}`);
  }

  // Group-level analytics, tags, search, activity
  async getTodayStats(range: TodayRange = 'today') {
    return this.request<TodayStatsResponse>(`/api/analytics/today${buildQuery({ range })}`);
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
