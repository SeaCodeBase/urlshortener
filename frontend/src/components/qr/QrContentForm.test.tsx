import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Link } from "@/types";
import { QrContentForm, emptyQrContentDraft, type QrContentDraft } from "./QrContentForm";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/dashboard/qr-codes/create",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("@/lib/hooks", () => ({
  useMeta: () => ({ data: { base_url: "http://localhost:8080" } }),
  useActiveGroupId: () => 7,
  useGroups: () => ({ data: [] }),
  useDomains: () => ({ data: [] }),
  useTags: () => ({ data: [] }),
}));

function Harness({ attachedLink, mode }: { attachedLink?: Link; mode?: "create" | "edit" }) {
  const [value, setValue] = useState<QrContentDraft>(emptyQrContentDraft());
  return <QrContentForm value={value} onChange={setValue} attachedLink={attachedLink} mode={mode} errors={{}} />;
}

const link = {
  id: 4,
  short_url: "https://sho.rt/abc123",
  original_url: "https://example.com/menu",
} as Link;

describe("QrContentForm", () => {
  it("shows the back-half and Generate only while sharing as a short link", async () => {
    render(<Harness />);
    expect(screen.queryByLabelText("Back-half (optional)")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Generate" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("switch", { name: "Share as a short link" }));
    expect(screen.getByLabelText("Back-half (optional)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Generate" })).toBeInTheDocument();
  });

  it("with an attached link shows a read-only panel instead of the destination", () => {
    render(<Harness attachedLink={link} />);
    expect(screen.getByText(/This QR Code will be attached to/)).toHaveTextContent(
      "This QR Code will be attached to sho.rt/abc123",
    );
    expect(screen.getByText("https://example.com/menu")).toBeInTheDocument();
    expect(screen.queryByLabelText("Destination URL")).not.toBeInTheDocument();
    expect(screen.queryByRole("switch", { name: "Share as a short link" })).not.toBeInTheDocument();
  });

  it("in edit mode keeps the destination editable but hides domain, share and UTM controls", () => {
    render(<Harness mode="edit" />);
    expect(screen.getByLabelText("Destination URL")).toBeInTheDocument();
    expect(screen.queryByLabelText("Domain")).not.toBeInTheDocument();
    expect(screen.queryByRole("switch", { name: "Share as a short link" })).not.toBeInTheDocument();
    expect(screen.queryByRole("switch", { name: "UTM parameters" })).not.toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "Code expiration" })).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "Dynamic routing" })).toBeInTheDocument();
  });
});
