import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { QrCode } from "@/types";
import { QrReadyDialog } from "./QrReadyDialog";

vi.mock("@/components/qr/QrPreview", () => ({
  QrPreview: ({ data }: { data: string }) => <div data-testid="qr-preview">{data}</div>,
}));

const qr = {
  id: 9,
  link_id: 4,
  render_customizations: {},
  short_url: "https://sho.rt/abc123",
  qr_url: "https://sho.rt/abc123?r=qr",
} as QrCode;

describe("QrReadyDialog", () => {
  it("shows labelled View code details and Download buttons", () => {
    render(<QrReadyDialog open onOpenChange={vi.fn()} qr={qr} />);
    expect(screen.getByRole("heading", { name: "Your code is ready!" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View code details" })).toHaveAttribute("href", "/dashboard/qr-codes/9");
    const download = screen.getByRole("button", { name: "Download" });
    expect(download).toHaveTextContent("Download");
    expect(screen.getByRole("link", { name: "Customize design" })).toHaveAttribute(
      "href",
      "/dashboard/qr-codes/9/customize",
    );
  });
});
