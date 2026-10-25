import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { api } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import type { BulkUpload, BulkUploadStatus } from "@/types";
import { bulkJobFinished, bulkPollInterval, useBulkUploads } from "./useBulkUploads";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

function job(id: number, status: BulkUploadStatus): BulkUpload {
  return {
    id,
    user_id: 1,
    group_id: 1,
    filename: `f${id}.csv`,
    status,
    total: 10,
    processed: status === "done" ? 10 : 3,
    succeeded: 3,
    failed: 0,
    error: "",
    created_at: "2026-10-25T00:00:00Z",
    updated_at: "2026-10-25T00:00:00Z",
  };
}

describe("bulkPollInterval", () => {
  it("polls every 3 s while a job is queued or processing", () => {
    expect(bulkPollInterval([job(1, "done"), job(2, "queued")])).toBe(3000);
    expect(bulkPollInterval([job(1, "processing")])).toBe(3000);
  });
  it("stops when every job is finished or there is no data", () => {
    expect(bulkPollInterval([job(1, "done"), job(2, "failed")])).toBe(false);
    expect(bulkPollInterval([])).toBe(false);
    expect(bulkPollInterval(undefined)).toBe(false);
  });
});

describe("bulkJobFinished", () => {
  it("is true when a job seen running is now done or failed", () => {
    const prev = new Map([[1, "processing" as const], [2, "queued" as const]]);
    expect(bulkJobFinished(prev, [job(1, "done"), job(2, "queued")])).toBe(true);
    expect(bulkJobFinished(prev, [job(1, "processing"), job(2, "failed")])).toBe(true);
    expect(bulkJobFinished(prev, [job(1, "processing"), job(2, "queued")])).toBe(false);
  });
  it("ignores the first load and jobs that were already finished", () => {
    expect(bulkJobFinished(null, [job(1, "done")])).toBe(false);
    expect(bulkJobFinished(new Map([[1, "done" as const]]), [job(1, "done")])).toBe(false);
  });
  it("counts a job that appeared already finished after the first load", () => {
    expect(bulkJobFinished(new Map(), [job(3, "done")])).toBe(true);
  });
});

describe("useBulkUploads", () => {
  let prevGroup: number | null;
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    prevGroup = useAuthStore.getState().activeGroupId;
    useAuthStore.setState({ activeGroupId: 1 });
  });
  afterEach(() => {
    useAuthStore.setState({ activeGroupId: prevGroup });
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("refetches every 3 s until no job is queued or processing", async () => {
    const list = vi
      .spyOn(api, "listBulkUploads")
      .mockResolvedValueOnce({ uploads: [job(1, "processing")] })
      .mockResolvedValue({ uploads: [job(1, "done")] });
    const { result } = renderHook(() => useBulkUploads(), { wrapper });
    await waitFor(() => expect(result.current.data?.[0].status).toBe("processing"));
    expect(list).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    await waitFor(() => expect(result.current.data?.[0].status).toBe("done"));
    expect(list).toHaveBeenCalledTimes(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    expect(list).toHaveBeenCalledTimes(2);
  });

  it("invalidates the links lists when polling sees a job finish", async () => {
    vi.spyOn(api, "listBulkUploads")
      .mockResolvedValueOnce({ uploads: [job(1, "processing")] })
      .mockResolvedValue({ uploads: [job(1, "done")] });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    const { result } = renderHook(() => useBulkUploads(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={qc}>{children}</QueryClientProvider>
      ),
    });
    await waitFor(() => expect(result.current.data?.[0].status).toBe("processing"));
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: ["links"] });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    await waitFor(() => expect(result.current.data?.[0].status).toBe("done"));
    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ["links"] }));
    expect(invalidate.mock.calls.filter((c) => JSON.stringify(c[0]) === '{"queryKey":["links"]}')).toHaveLength(1);
  });

  it("does not invalidate links when the first load shows finished jobs", async () => {
    vi.spyOn(api, "listBulkUploads").mockResolvedValue({ uploads: [job(1, "done")] });
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    const { result } = renderHook(() => useBulkUploads(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={qc}>{children}</QueryClientProvider>
      ),
    });
    await waitFor(() => expect(result.current.data?.[0].status).toBe("done"));
    expect(invalidate).not.toHaveBeenCalled();
  });
});
