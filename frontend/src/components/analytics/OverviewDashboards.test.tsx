import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Dashboard } from "@/types";
import { DEFAULT_LAYOUT } from "./modules";
import { OverviewDashboards } from "./OverviewDashboards";

const push = vi.fn();
const mutateAsync = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
}));

function dashboard(id: number, name: string, updatedAt: string): Dashboard {
  return {
    id,
    group_id: 7,
    name,
    is_default: id === 1,
    date_range: "last_7_days",
    filter: { type: "all" },
    layout: DEFAULT_LAYOUT,
    created_by: { id: 1, name: "Jose" },
    created_at: updatedAt,
    updated_at: updatedAt,
  };
}

const dashboards = [
  dashboard(4, "Campaign", "2026-10-24T12:00:00Z"),
  dashboard(3, "Menus", "2026-10-20T12:00:00Z"),
  dashboard(2, "Social", "2026-09-02T12:00:00Z"),
  dashboard(1, "Default", "2026-08-01T12:00:00Z"),
];

const list = { isError: false, refetch: vi.fn() };
vi.mock("@/lib/hooks", () => ({
  useDashboards: () =>
    list.isError
      ? { data: undefined, isLoading: false, isError: true, refetch: list.refetch }
      : { data: { dashboards, total: dashboards.length }, isLoading: false, isError: false, refetch: list.refetch },
  useDashboardMutations: () => ({ create: { mutateAsync, isPending: false } }),
}));

describe("OverviewDashboards", () => {
  beforeEach(() => {
    push.mockReset();
    mutateAsync.mockReset();
    list.isError = false;
    list.refetch.mockReset();
  });

  it("renders the create tile, at most 3 dashboard tiles and View all", () => {
    render(<OverviewDashboards />);
    expect(screen.getByRole("button", { name: "+ Create dashboard" })).toBeInTheDocument();

    const tiles = screen.getAllByRole("link", { name: /Modified on/ });
    expect(tiles).toHaveLength(3);
    expect(tiles[0]).toHaveAttribute("href", "/dashboard/analytics/dashboards/4");
    expect(within(tiles[0]).getByText("Campaign")).toBeInTheDocument();
    expect(within(tiles[0]).getByText("Modified on Oct 24, 2026")).toBeInTheDocument();
    expect(screen.queryByText("Default")).not.toBeInTheDocument();

    expect(screen.getByRole("link", { name: "View all →" })).toHaveAttribute("href", "/dashboard/analytics/dashboards");
  });

  it("creates a dashboard with the typed name and opens it", async () => {
    mutateAsync.mockResolvedValue(dashboard(9, "Q4 launch", "2026-10-25T00:00:00Z"));
    const user = userEvent.setup();
    render(<OverviewDashboards />);

    await user.click(screen.getByRole("button", { name: "+ Create dashboard" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("heading", { name: "Create dashboard" })).toBeInTheDocument();
    const submit = within(dialog).getByRole("button", { name: "Create dashboard" });
    expect(submit).toBeDisabled();

    await user.type(within(dialog).getByLabelText("Dashboard name"), "  Q4 launch  ");
    await user.click(submit);

    expect(mutateAsync).toHaveBeenCalledWith("Q4 launch");
    expect(push).toHaveBeenCalledWith("/dashboard/analytics/dashboards/9");
  });

  it("shows an error with Retry when the list fails", async () => {
    list.isError = true;
    const user = userEvent.setup();
    render(<OverviewDashboards />);
    expect(screen.getByText("Couldn't load dashboards.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "+ Create dashboard" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(list.refetch).toHaveBeenCalled();
  });
});
