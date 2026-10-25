import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { api } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import type { BulkUpload, BulkUploadStatus } from "@/types";
import { bulkPollInterval, useBulkUploads } from "./useBulkUploads";

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
});
