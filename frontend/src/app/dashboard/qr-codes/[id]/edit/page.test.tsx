import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { QrCode } from "@/types";
import EditQrCodePage from "./page";

const push = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
  useParams: () => ({ id: "5" }),
  usePathname: () => "/dashboard/qr-codes/5/edit",
  useSearchParams: () => new URLSearchParams(),
}));

const toastSuccess = vi.fn();
vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: vi.fn() } }));

const qr = {
  id: 5,
  link_id: 9,
  title: "Menu",
  tags: ["food"],
  original_url: "https://example.com/menu",
  short_url: "https://sho.rt/abc",
  qr_url: "https://sho.rt/abc?r=qr",
  expires_at: "2099-01-01T00:00:00Z",
  link_source: "qr",
  render_customizations: {},
  link: { routing_rules: [] },
} as unknown as QrCode;

const mutateAsync = vi.fn();
vi.mock("@/lib/hooks", () => ({
  useQrCode: () => ({ data: qr, isLoading: false, isError: false }),
  useUpdateQrCode: () => ({ mutateAsync, isPending: false }),
  useMeta: () => ({ data: { base_url: "http://localhost:8080" } }),
  useActiveGroupId: () => 7,
  useGroups: () => ({ data: [] }),
  useDomains: () => ({ data: [] }),
  useTags: () => ({ data: [] }),
}));

describe("EditQrCodePage", () => {
  beforeEach(() => {
    push.mockReset();
    toastSuccess.mockReset();
    mutateAsync.mockReset().mockResolvedValue(qr);
  });

  it("prefills the form and saves only the changed fields", async () => {
    const user = userEvent.setup();
    render(<EditQrCodePage />);

    expect(screen.getByRole("heading", { name: "Edit QR Code" })).toBeInTheDocument();
    const destination = screen.getByLabelText("Destination URL");
    expect(destination).toHaveValue("https://example.com/menu");
    expect(screen.getByLabelText("Title (optional)")).toHaveValue("Menu");
    expect(screen.queryByRole("switch", { name: "Share as a short link" })).not.toBeInTheDocument();

    await user.clear(destination);
    await user.type(destination, "https://example.com/new-menu");
    await user.click(screen.getByRole("switch", { name: "Code expiration" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(mutateAsync).toHaveBeenCalledWith({
      id: 5,
      data: { long_url: "https://example.com/new-menu", clear_expiration: true },
    });
    expect(toastSuccess).toHaveBeenCalledWith("QR Code updated");
    expect(push).toHaveBeenCalledWith("/dashboard/qr-codes/5");
  });
});
