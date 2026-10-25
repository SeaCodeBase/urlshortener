import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiError } from "@/lib/api";
import { BulkUploadForm } from "./BulkUploadForm";

const upload = vi.fn();
const toastSuccess = vi.fn();
const downloadTemplate = vi.fn();

vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: vi.fn() } }));
vi.mock("@/lib/hooks", () => ({
  useUploadBulkCsv: () => ({ mutateAsync: upload, isPending: false }),
}));
vi.mock("./bulkTemplate", () => ({ downloadBulkTemplate: () => downloadTemplate() }));

function csv(name: string, bytes = 20) {
  return new File(["x".repeat(bytes)], name, { type: "text/csv" });
}

describe("BulkUploadForm", () => {
  beforeEach(() => {
    upload.mockReset().mockResolvedValue({ id: 1 });
    toastSuccess.mockReset();
    downloadTemplate.mockReset();
  });

  it("keeps Upload disabled until a file is chosen", () => {
    render(<BulkUploadForm />);
    expect(screen.getByRole("button", { name: "Upload" })).toBeDisabled();
    expect(screen.getByLabelText("CSV file")).toHaveAttribute("accept", ".csv,text/csv");
    expect(screen.getByText(/Save as CSV UTF-8\./)).toBeInTheDocument();
  });

  it("rejects a 3 MB file with an inline message and does not upload", async () => {
    const user = userEvent.setup();
    render(<BulkUploadForm />);
    await user.upload(screen.getByLabelText("CSV file"), csv("big.csv", 3 * 1024 * 1024));
    expect(screen.getByText("File is larger than 2 MB")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Upload" })).toBeDisabled();
    expect(upload).not.toHaveBeenCalled();
  });

  it("rejects a non-.csv file", async () => {
    const user = userEvent.setup({ applyAccept: false });
    render(<BulkUploadForm />);
    await user.upload(screen.getByLabelText("CSV file"), new File(["a"], "links.txt", { type: "text/plain" }));
    expect(screen.getByText("Choose a .csv file")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Upload" })).toBeDisabled();
  });

  it("uploads the chosen file, toasts and resets the input", async () => {
    const user = userEvent.setup();
    render(<BulkUploadForm />);
    const file = csv("links.csv");
    const input = screen.getByLabelText("CSV file") as HTMLInputElement;
    await user.upload(input, file);
    await user.click(screen.getByRole("button", { name: "Upload" }));
    expect(upload).toHaveBeenCalledWith(file);
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Upload queued"));
    expect(input.value).toBe("");
    expect(screen.getByRole("button", { name: "Upload" })).toBeDisabled();
  });

  it("shows an ApiError inline", async () => {
    upload.mockRejectedValue(new ApiError("at most 1000 rows", 400));
    const user = userEvent.setup();
    render(<BulkUploadForm />);
    await user.upload(screen.getByLabelText("CSV file"), csv("links.csv"));
    await user.click(screen.getByRole("button", { name: "Upload" }));
    expect(await screen.findByText("at most 1000 rows")).toBeInTheDocument();
  });

  it("downloads the template", async () => {
    const user = userEvent.setup();
    render(<BulkUploadForm />);
    await user.click(screen.getByRole("button", { name: "Download template" }));
    expect(downloadTemplate).toHaveBeenCalled();
  });
});
