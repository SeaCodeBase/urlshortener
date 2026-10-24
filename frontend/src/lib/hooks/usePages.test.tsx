import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { api } from "@/lib/api";
import { useSlugCheck } from "./usePages";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("useSlugCheck", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.spyOn(api, "checkSlug").mockResolvedValue({ slug: "jose", available: true, reason: "" });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("debounces and queries a valid slug", async () => {
    const { result, rerender } = renderHook(({ s }) => useSlugCheck(s), { wrapper, initialProps: { s: "jo" } });
    rerender({ s: "jose" });
    expect(result.current.isChecking).toBe(true);
    expect(api.checkSlug).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(300));
    await waitFor(() => expect(result.current.data).toEqual({ slug: "jose", available: true, reason: "" }));
    expect(api.checkSlug).toHaveBeenCalledTimes(1);
    expect(api.checkSlug).toHaveBeenCalledWith("jose");
  });

  it("answers invalid and reserved slugs locally", async () => {
    const { result, rerender } = renderHook(({ s }) => useSlugCheck(s), { wrapper, initialProps: { s: "-bad" } });
    expect(result.current.data).toEqual({ slug: "-bad", available: false, reason: "invalid" });
    rerender({ s: "admin" });
    act(() => vi.advanceTimersByTime(300));
    await waitFor(() => expect(result.current.data?.reason).toBe("reserved"));
    expect(result.current.data?.available).toBe(false);
    expect(api.checkSlug).not.toHaveBeenCalled();
  });

  it("returns null data for an empty slug", () => {
    const { result } = renderHook(() => useSlugCheck(""), { wrapper });
    expect(result.current.data).toBeNull();
    expect(result.current.isChecking).toBe(false);
  });
});
