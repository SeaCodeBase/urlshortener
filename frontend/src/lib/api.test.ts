import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "./api";
import { useAuthStore } from "@/stores/auth";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("ApiClient", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
    api.setToken("tok-123");
    useAuthStore.getState().setActiveGroup(7);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    api.setToken(null);
    useAuthStore.setState({ activeGroupId: null });
  });

  function lastCall(): { url: string; init: RequestInit; headers: Headers } {
    const [url, init] = fetchMock.mock.calls.at(-1) as [string, RequestInit];
    return { url, init, headers: new Headers(init.headers) };
  }

  it("sends Authorization: Bearer and X-Group-ID", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ tags: ["a"] }));

    await api.getTags("link");

    const { url, headers } = lastCall();
    expect(url).toBe("/api/tags?type=link");
    expect(headers.get("Authorization")).toBe("Bearer tok-123");
    expect(headers.get("X-Group-ID")).toBe("7");
  });

  it("omits X-Group-ID when no group is active", async () => {
    useAuthStore.setState({ activeGroupId: null });
    fetchMock.mockResolvedValue(jsonResponse({ groups: [] }));

    await api.getGroups();

    expect(lastCall().headers.has("X-Group-ID")).toBe(false);
  });

  it("exportLinks returns a Blob with the same headers", async () => {
    fetchMock.mockResolvedValue(
      new Response("id,short_url\n1,x\n", {
        status: 200,
        headers: { "Content-Type": "text/csv" },
      }),
    );

    const blob = await api.exportLinks({ q: "foo", archived: "on" });

    expect(blob).toBeInstanceOf(Blob);
    expect(await blob.text()).toBe("id,short_url\n1,x\n");
    const { url, headers } = lastCall();
    expect(url).toBe("/api/links/export?q=foo&archived=on");
    expect(headers.get("Authorization")).toBe("Bearer tok-123");
    expect(headers.get("X-Group-ID")).toBe("7");
  });

  it("throws ApiError with the body message and status", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: "x" }, 409));

    const err = await api.deleteLink(5).catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toBeInstanceOf(Error);
    expect((err as ApiError).status).toBe(409);
    expect((err as ApiError).message).toBe("x");
  });

  it("falls back to a generic message when the error body is not JSON", async () => {
    fetchMock.mockResolvedValue(new Response("oops", { status: 500 }));

    const err = (await api.getLink(1).catch((e: unknown) => e)) as ApiError;

    expect(err.status).toBe(500);
    expect(err.message).toBe("Request failed");
  });

  it("getLinks builds the query string, repeating tags and skipping empty values", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ links: [], total: 0, page: 2, total_pages: 0 }));

    await api.getLinks({
      q: "shop",
      tags: ["a", "b c"],
      archived: "off",
      custom_back_half: "on",
      expiration: "expiring",
      has_qr: "both",
      created_after: "2026-01-01",
      created_before: undefined,
      sort: "clicks",
      page: 2,
      limit: 50,
    });

    expect(lastCall().url).toBe(
      "/api/links?q=shop&tags=a&tags=b+c&archived=off&custom_back_half=on&expiration=expiring&has_qr=both&created_after=2026-01-01&sort=clicks&page=2&limit=50",
    );
  });

  it("getLinkStats passes the stats query", async () => {
    fetchMock.mockResolvedValue(jsonResponse({}));

    await api.getLinkStats(3, { from: "2026-10-01", to: "2026-10-24", unit: "day", compare: "previous", source: "all" });

    expect(lastCall().url).toBe(
      "/api/links/3/stats?from=2026-10-01&to=2026-10-24&unit=day&compare=previous&source=all",
    );
  });

  it("bulkLinks sends a PATCH with the JSON body", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ updated: 2 }));

    const res = await api.bulkLinks({ action: "edit_tags", ids: [1, 2], add_tags: ["x"] });

    const { url, init } = lastCall();
    expect(url).toBe("/api/links/bulk");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body as string)).toEqual({ action: "edit_tags", ids: [1, 2], add_tags: ["x"] });
    expect(res.updated).toBe(2);
  });

  it("returns an empty object on 204", async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }));

    await expect(api.deleteLink(9)).resolves.toEqual({});
  });
  it("wraps fetch rejections as ApiError('Network error', 0)", async () => {
    fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));

    const err = await api.getTags().catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ message: "Network error", status: 0 });
  });

  it("a 401 signs the user out once and rethrows", async () => {
    const original = useAuthStore.getState().logout;
    const spy = vi.fn(original);
    useAuthStore.setState({ token: "tok-123", logout: spy });
    fetchMock.mockImplementation(() => Promise.resolve(jsonResponse({ error: "unauthorized" }, 401)));

    const results = await Promise.allSettled([api.getTags(), api.getTags()]);

    expect(results.map((r) => r.status)).toEqual(["rejected", "rejected"]);
    expect((results[0] as PromiseRejectedResult).reason).toMatchObject({ status: 401 });
    expect(spy).toHaveBeenCalledTimes(1);
    expect(useAuthStore.getState().token).toBeNull();
    useAuthStore.setState({ logout: original });
  });

  it("a 401 without a token (failed login) does not sign out", async () => {
    api.setToken(null);
    const spy = vi.fn();
    const original = useAuthStore.getState().logout;
    useAuthStore.setState({ logout: spy });
    fetchMock.mockResolvedValue(jsonResponse({ error: "invalid credentials" }, 401));

    await expect(api.login("a@b.c", "x")).rejects.toMatchObject({ status: 401 });
    expect(spy).not.toHaveBeenCalled();
    useAuthStore.setState({ logout: original });
  });
});
