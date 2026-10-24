import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { Link } from "@/types";
import { LinkCard } from "./LinkCard";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/dashboard/links",
  useSearchParams: () => new URLSearchParams(),
}));

const base: Link = {
  id: 7,
  user_id: 1,
  group_id: 1,
  short_code: "abc123",
  original_url: "https://example.com/some/long/path",
  title: "Example page",
  expires_at: null,
  is_active: true,
  archived: false,
  has_custom_back_half: false,
  redirect_count: 0,
  source: "link",
  created_at: "2026-10-20T10:00:00Z",
  updated_at: "2026-10-20T10:00:00Z",
  short_url: "https://sho.rt/abc123",
  tags: [],
  total_clicks: 3,
  favicon_url: "",
};

function wrap(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <TooltipProvider>{ui}</TooltipProvider>
    </QueryClientProvider>,
  );
}

async function openMenu() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "More" }));
  const menu = await screen.findByRole("menu");
  return { user, menu };
}

describe("LinkCard", () => {
  it("renders title, short url, destination, engagements and tags", () => {
    wrap(<LinkCard link={base} selected={false} onSelectedChange={vi.fn()} />);
    expect(screen.getByRole("link", { name: "Example page" })).toHaveAttribute("href", "/dashboard/links/7");
    const short = screen.getByRole("link", { name: "sho.rt/abc123" });
    expect(short).toHaveAttribute("href", "https://sho.rt/abc123");
    expect(short).toHaveAttribute("target", "_blank");
    expect(screen.getByText("https://example.com/some/long/path")).toBeInTheDocument();
    expect(screen.getByText("3 engagements")).toBeInTheDocument();
    expect(screen.getByText("No tags")).toBeInTheDocument();
    expect(screen.getByText("Add tag")).toBeInTheDocument();
    expect(screen.queryByText("Hidden")).not.toBeInTheDocument();
    for (const name of ["Edit link", "Share", "View details", "More"]) {
      expect(screen.getByRole(name === "Edit link" || name === "View details" ? "link" : "button", { name })).toBeInTheDocument();
    }
  });

  it("uses the singular for one engagement", () => {
    wrap(<LinkCard link={{ ...base, total_clicks: 1 }} selected={false} onSelectedChange={vi.fn()} />);
    expect(screen.getByText("1 engagement")).toBeInTheDocument();
  });

  it("shows a Hidden badge when archived", () => {
    wrap(<LinkCard link={{ ...base, archived: true }} selected={false} onSelectedChange={vi.fn()} />);
    expect(screen.getByText("Hidden")).toBeInTheDocument();
  });

  it("actions menu lists the Bitly items in order", async () => {
    wrap(<LinkCard link={base} selected={false} onSelectedChange={vi.fn()} />);
    const { menu } = await openMenu();
    const items = within(menu).getAllByRole("menuitem").map((i) => i.textContent?.trim());
    expect(items).toEqual(["View link details", "Create QR Code", "Add to a page", "Add to campaign", "Hide link", "Delete"]);
    expect(within(menu).getByRole("menuitem", { name: "Delete" })).not.toHaveAttribute("data-disabled");
  });

  it("lists View QR Code linking to the link's QR Code", async () => {
    wrap(<LinkCard link={{ ...base, qr_code_id: 9 }} selected={false} onSelectedChange={vi.fn()} />);
    const { menu } = await openMenu();
    expect(within(menu).getByRole("menuitem", { name: "View QR Code" })).toHaveAttribute("href", "/dashboard/qr-codes/9");
    expect(within(menu).queryByRole("menuitem", { name: "Create QR Code" })).not.toBeInTheDocument();
  });

  it.each([
    ["without", base, "This permanently deletes the link and its click data."],
    ["with", { ...base, qr_code_id: 9 }, "This permanently deletes the link, its click data and its QR Code."],
  ])("delete confirmation copy %s a QR Code", async (_label, link, copy) => {
    wrap(<LinkCard link={link} selected={false} onSelectedChange={vi.fn()} />);
    const { user, menu } = await openMenu();
    await user.click(within(menu).getByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText(copy)).toBeInTheDocument();
  });

  it("offers Show link for hidden links", async () => {
    wrap(<LinkCard link={{ ...base, archived: true }} selected={false} onSelectedChange={vi.fn()} />);
    const { menu } = await openMenu();
    expect(within(menu).getByRole("menuitem", { name: "Show link" })).toBeInTheDocument();
    expect(within(menu).queryByRole("menuitem", { name: "Hide link" })).not.toBeInTheDocument();
  });

  it.each([
    ["redirected", { redirect_count: 2 }],
    ["custom back-half", { has_custom_back_half: true }],
  ])("disables Delete with a tooltip when the link is %s", async (_label, patch) => {
    wrap(<LinkCard link={{ ...base, ...patch }} selected={false} onSelectedChange={vi.fn()} />);
    const { user, menu } = await openMenu();
    const del = within(menu).getByRole("menuitem", { name: "Delete" });
    expect(del).toHaveAttribute("data-disabled");
    await user.hover(del);
    expect((await screen.findAllByText("This link can't be deleted. You can hide it instead.")).length).toBeGreaterThan(0);
  });
});
