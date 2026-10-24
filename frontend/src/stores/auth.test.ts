import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthStore } from "./auth";
import type { Group, User } from "@/types";

const group = (id: number): Group => ({
  id,
  name: `G${id}`,
  default_domain_id: null,
  created_at: "2026-10-24T00:00:00Z",
  updated_at: "2026-10-24T00:00:00Z",
});

const user: User = {
  id: 1,
  email: "a@b.c",
  account_name: null,
  default_group_id: 2,
  home_checklist_dismissed: false,
  created_at: "2026-10-24T00:00:00Z",
  updated_at: "2026-10-24T00:00:00Z",
};

function respond(url: string): Response {
  const body = url.endsWith("/api/auth/me") ? user : { groups: [group(1), group(2)] };
  return new Response(JSON.stringify(body), { status: 200 });
}

describe("useAuthStore", () => {
  const fetchMock = vi.fn((url: string) => Promise.resolve(respond(url)));

  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
    useAuthStore.setState({ user: null, token: "t", activeGroupId: null, groups: [] });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    useAuthStore.getState().logout();
  });

  it("checkAuth loads groups and defaults to user.default_group_id", async () => {
    await useAuthStore.getState().checkAuth();

    const s = useAuthStore.getState();
    expect(s.user?.id).toBe(1);
    expect(s.groups.map((g) => g.id)).toEqual([1, 2]);
    expect(s.activeGroupId).toBe(2);
    expect(s.isLoading).toBe(false);
  });

  it("keeps a still-valid active group", async () => {
    useAuthStore.setState({ activeGroupId: 1 });

    await useAuthStore.getState().checkAuth();

    expect(useAuthStore.getState().activeGroupId).toBe(1);
  });

  it("replaces an unknown active group", async () => {
    useAuthStore.setState({ activeGroupId: 99 });

    await useAuthStore.getState().checkAuth();

    expect(useAuthStore.getState().activeGroupId).toBe(2);
  });
});
