import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { BulkUpload } from "@/types";
import { BulkJobRow } from "./BulkJobRow";

const downloadResults = vi.fn();
const downloadBlob = vi.fn();
const toastError = vi.fn();

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: (m: string) => toastError(m) } }));
vi.mock("@/lib/api", async (orig) => ({
  ...(await orig<typeof import("@/lib/api")>()),
  api: { downloadBulkResults: (id: number) => downloadResults(id) },
}));
vi.mock("@/lib/download", () => ({ downloadBlob: (...a: unknown[]) => downloadBlob(...a) }));

const job: BulkUpload = {
  id: 42,
  user_id: 1,
  group_id: 3,
  filename: "spring.csv",
  status: "processing",
  total: 10,
  processed: 4,
  succeeded: 3,
  failed: 1,
  error: "",
  created_at: "2026-10-25T10:00:00Z",
  updated_at: "2026-10-25T10:01:00Z",
};

function renderRow(u: BulkUpload) {
  return render(
    <table>
      <tbody>
        <BulkJobRow upload={u} />
      </tbody>
    </table>,
  );
}

describe("BulkJobRow", () => {
  beforeEach(() => {
    downloadResults.mockReset();
    downloadBlob.mockReset();
    toastError.mockReset();
  });

  it("shows progress with a bar and disables the download while processing", () => {
    renderRow(job);
    expect(screen.getByText("spring.csv")).toBeInTheDocument();
    expect(screen.getByText("Processing")).toBeInTheDocument();
    expect(screen.getByText("4/10 · 3 ok · 1 failed")).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Download results CSV" })).toBeDisabled();
  });

  it("enables the download when done and saves the blob", async () => {
    const blob = new Blob(["row,long_url"], { type: "text/csv" });
    downloadResults.mockResolvedValue(blob);
    const user = userEvent.setup();
    renderRow({ ...job, status: "done", processed: 10, succeeded: 9 });
    expect(screen.getByText("Done")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Download results CSV" }));
    expect(downloadResults).toHaveBeenCalledWith(42);
    await waitFor(() => expect(downloadBlob).toHaveBeenCalledWith("bulk-42-results.csv", blob));
  });

  it("toasts when the download fails", async () => {
    downloadResults.mockRejectedValue(new Error("boom"));
    const user = userEvent.setup();
    renderRow({ ...job, status: "failed" });
    expect(screen.getByText("Failed")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Download results CSV" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("boom"));
    expect(downloadBlob).not.toHaveBeenCalled();
  });
});
