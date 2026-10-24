import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { Dashboard, DashboardFilter } from "@/types";
import { DEFAULT_LAYOUT } from "./modules";
import { DashboardRow, filterSummary } from "./DashboardRow";

const update = vi.fn();
const duplicate = vi.fn();
const remove = vi.fn();

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/hooks", () => ({
  useDashboardMutations: () => ({
    update: { mutateAsync: update, isPending: false },
    duplicate: { mutateAsync: duplicate, isPending: false },
    remove: { mutateAsync: remove, isPending: false },
  }),
}));

function dashboard(over: Partial<Dashboard> = {}): Dashboard {
  return {
    id: 3,
    group_id: 7,
    name: "Campaign",
    is_default: false,
    date_range: "last_30_days",
    filter: { type: "all" },
    layout: DEFAULT_LAYOUT,
    created_by: { id: 1, name: "jose" },
    created_at: "2026-10-01T12:00:00Z",
    updated_at: "2026-10-24T12:00:00Z",
    ...over,
  };
}

function renderRow(d: Dashboard) {
  return render(
    <TooltipProvider>
      <DashboardRow dashboard={d} />
    </TooltipProvider>,
  );
}

describe("DashboardRow", () => {
  beforeEach(() => {
    update.mockReset();
    duplicate.mockReset();
    remove.mockReset();
  });

  it("renders creator, name link, dates and date range", () => {
    renderRow(dashboard());
    expect(screen.getByText("J")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Campaign" })).toHaveAttribute("href", "/dashboard/analytics/dashboards/3");
    expect(screen.getByText("Modified on Oct 24, 2026")).toBeInTheDocument();
    expect(screen.getByText("Created by jose")).toBeInTheDocument();
    expect(screen.getByText("Last 30 days")).toBeInTheDocument();
  });

  it.each<[DashboardFilter, string]>([
    [{ type: "all" }, "Showing data for all links and QR Codes"],
    [{ type: "links", ids: [1, 2, 3] }, "Showing data for 3 links"],
    [{ type: "qr", ids: [5, 6] }, "Showing data for 2 QR Codes"],
  ])("summarises filter %j", (filter, text) => {
    renderRow(dashboard({ filter }));
    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it("disables Delete for the default dashboard", async () => {
    const user = userEvent.setup();
    renderRow(dashboard({ is_default: true, name: "Default" }));
    await user.click(screen.getByRole("button", { name: "More" }));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: /Delete/ })).toHaveAttribute("aria-disabled", "true");
    expect(within(menu).getByRole("menuitem", { name: /Rename/ })).not.toHaveAttribute("aria-disabled");
  });

  it("deletes a non-default dashboard after confirming", async () => {
    remove.mockResolvedValue(undefined);
    const user = userEvent.setup();
    renderRow(dashboard());
    await user.click(screen.getByRole("button", { name: "More" }));
    await user.click(await screen.findByRole("menuitem", { name: /Delete/ }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Delete dashboard?")).toBeInTheDocument();
    expect(within(dialog).getByText("This permanently deletes the dashboard.")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));
    expect(remove).toHaveBeenCalledWith(3);
  });

  it("renames via the name dialog", async () => {
    update.mockResolvedValue(dashboard({ name: "Q4" }));
    const user = userEvent.setup();
    renderRow(dashboard());
    await user.click(screen.getByRole("button", { name: "More" }));
    await user.click(await screen.findByRole("menuitem", { name: /Rename/ }));
    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByLabelText("Dashboard name");
    expect(input).toHaveValue("Campaign");
    await user.clear(input);
    await user.type(input, "Q4");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(update).toHaveBeenCalledWith({ id: 3, patch: { name: "Q4" } });
  });

  it("duplicates", async () => {
    duplicate.mockResolvedValue(dashboard({ id: 4 }));
    const user = userEvent.setup();
    renderRow(dashboard());
    await user.click(screen.getByRole("button", { name: "More" }));
    await user.click(await screen.findByRole("menuitem", { name: /Duplicate/ }));
    expect(duplicate).toHaveBeenCalledWith(3);
  });
});

describe("filterSummary", () => {
  it("counts the filter ids", () => {
    expect(filterSummary({ type: "all" })).toBe("Showing data for all links and QR Codes");
    expect(filterSummary({ type: "links", ids: [1] })).toBe("Showing data for 1 link");
    expect(filterSummary({ type: "qr", ids: [4, 5] })).toBe("Showing data for 2 QR Codes");
  });

  it("reads a filter whose ids were all deleted as 0", () => {
    expect(filterSummary({ type: "links", ids: [] })).toBe("Showing data for 0 links");
    // The API omits an empty ids array.
    expect(filterSummary({ type: "qr" } as DashboardFilter)).toBe("Showing data for 0 QR Codes");
  });
});
