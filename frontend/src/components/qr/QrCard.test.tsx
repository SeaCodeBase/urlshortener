import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { QrCode } from "@/types";
import { QrCard } from "./QrCard";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/dashboard/qr-codes",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("@/components/qr/QrPreview", () => ({
  QrPreview: ({ data }: { data: string }) => <div data-testid="qr-preview">{data}</div>,
}));

const base: QrCode = {
  id: 9,
  user_id: 1,
  group_id: 1,
  link_id: 4,
  title: "Menu code",
  render_customizations: {},
  archived: false,
  created_at: "2026-10-20T10:00:00Z",
  updated_at: "2026-10-20T10:00:00Z",
  tags: [],
  total_scans: 3,
  short_url: "https://sho.rt/abc123",
  qr_url: "https://sho.rt/abc123?r=qr",
  original_url: "https://example.com/menu",
  favicon_url: "",
  expires_at: null,
  link_source: "qr",
  image_url: "/api/qr-codes/9/image",
};

function wrap(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("QrCard", () => {
  it("renders preview, title, type, destination, scans and actions", () => {
    wrap(<QrCard qr={base} selected={false} onSelect={vi.fn()} />);
    expect(screen.getByTestId("qr-preview")).toHaveTextContent("https://sho.rt/abc123?r=qr");
    expect(screen.getByRole("link", { name: "Menu code" })).toHaveAttribute("href", "/dashboard/qr-codes/9");
    expect(screen.getByText("Website")).toBeInTheDocument();
    expect(screen.getByText("https://example.com/menu")).toBeInTheDocument();
    expect(screen.getByText("3 scans")).toBeInTheDocument();
    expect(screen.getByText("No tags")).toBeInTheDocument();
    expect(screen.queryByText("sho.rt/abc123")).not.toBeInTheDocument();
    expect(screen.queryByText("Hidden")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Edit" })).toHaveAttribute("href", "/dashboard/qr-codes/9/edit");
    expect(screen.getByRole("link", { name: "Customize" })).toHaveAttribute("href", "/dashboard/qr-codes/9/customize");
    expect(screen.getByRole("link", { name: "View details" })).toHaveAttribute("href", "/dashboard/qr-codes/9");
    expect(screen.getByRole("button", { name: "Download" })).toBeInTheDocument();
  });

  it("shows the short link only when the code is attached to a link", () => {
    wrap(<QrCard qr={{ ...base, link_source: "link" }} selected={false} onSelect={vi.fn()} />);
    expect(screen.getByText("sho.rt/abc123")).toBeInTheDocument();
  });

  it("falls back to the destination host, uses the singular and shows Hidden", () => {
    wrap(<QrCard qr={{ ...base, title: null, total_scans: 1, archived: true }} selected={false} onSelect={vi.fn()} />);
    expect(screen.getByRole("link", { name: "example.com" })).toBeInTheDocument();
    expect(screen.getByText("1 scan")).toBeInTheDocument();
    expect(screen.getByText("Hidden")).toBeInTheDocument();
  });

  it("actions menu lists the QR items", async () => {
    const user = userEvent.setup();
    wrap(<QrCard qr={base} selected={false} onSelect={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "More" }));
    const menu = await screen.findByRole("menu");
    const items = within(menu).getAllByRole("menuitem").map((i) => i.textContent?.trim());
    expect(items).toEqual(["View details", "Duplicate design", "Hide code", "Delete"]);
    expect(within(menu).getByRole("menuitem", { name: "Duplicate design" })).toHaveAttribute(
      "href",
      "/dashboard/qr-codes/create?design=9",
    );
  });

  it("offers Show code for hidden codes and download formats", async () => {
    const user = userEvent.setup();
    wrap(<QrCard qr={{ ...base, archived: true }} selected={false} onSelect={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "More" }));
    expect(await screen.findByRole("menuitem", { name: "Show code" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Download" }));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getAllByRole("menuitem").map((i) => i.textContent?.trim())).toEqual(["PNG", "SVG", "JPG"]);
  });

  it("explains that deleting a QR-owned code also deletes its short link", async () => {
    const user = userEvent.setup();
    wrap(<QrCard qr={base} selected={false} onSelect={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "More" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    expect(await screen.findByText("Delete QR Code?")).toBeInTheDocument();
    expect(screen.getByText("This permanently deletes the QR Code and its short link.")).toBeInTheDocument();
  });
});
