import { cloneElement, type ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CampaignStats } from "@/types";
import { rangeFor } from "@/components/analytics/modules";
import { CampaignPerformance } from "./CampaignPerformance";

// jsdom has no layout: give the chart a fixed size so recharts renders its legend.
vi.mock("recharts", async (importOriginal) => {
  const actual = await importOriginal<typeof import("recharts")>();
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: ReactElement<{ width?: number; height?: number }> }) =>
      cloneElement(children, { width: 600, height: 280 }),
  };
});

const state: { data: CampaignStats | undefined; isLoading: boolean; isError: boolean; calls: unknown[][] } = {
  data: undefined,
  isLoading: false,
  isError: false,
  calls: [],
};
vi.mock("@/lib/hooks", () => ({
  useCampaignStats: (id: unknown, query: unknown) => {
    state.calls.push([id, query]);
    return { data: state.data, isLoading: state.isLoading, isError: state.isError, refetch: vi.fn() };
  },
}));

const series = (c: number[]) => c.map((clicks, i) => ({ date: `2026-10-2${i}`, clicks }));

beforeEach(() => {
  state.calls = [];
  state.isLoading = false;
  state.isError = false;
  state.data = {
    totals: { links: 3, engagements: 1240 },
    channels: [
      { channel_id: 10, name: "Email", links: 2, engagements: 1000, series: series([1, 2, 3]) },
      { channel_id: 11, name: "Social", links: 1, engagements: 240, series: series([0, 1, 0]) },
    ],
  };
});

describe("CampaignPerformance", () => {
  it("queries the last 7 days by default and charts one line per channel", () => {
    const { container } = render(<CampaignPerformance campaignId={5} />);
    expect(screen.getByRole("heading", { name: "Performance" })).toBeInTheDocument();
    expect(state.calls.at(-1)).toEqual([5, { ...rangeFor("last_7_days"), unit: "day" }]);
    const legend = Array.from(container.querySelectorAll(".recharts-legend-item-text")).map((e) => e.textContent);
    expect(legend).toEqual(["Email", "Social"]);
  });

  it("renders a summary row per channel plus a Total row", () => {
    render(<CampaignPerformance campaignId={5} />);
    const table = screen.getByRole("table");
    const rows = within(table).getAllByRole("row");
    expect(rows.map((r) => r.textContent)).toEqual([
      "ChannelLinksEngagements",
      "Email21,000",
      "Social1240",
      "Total31,240",
    ]);
  });

  it("changes the preset", async () => {
    const user = userEvent.setup();
    render(<CampaignPerformance campaignId={5} />);
    await user.click(screen.getByRole("combobox", { name: "Date range" }));
    await user.click(await screen.findByRole("option", { name: "Last 30 days" }));
    expect(state.calls.at(-1)).toEqual([5, { ...rangeFor("last_30_days"), unit: "day" }]);
  });

  it("shows an error", () => {
    state.data = undefined;
    state.isError = true;
    render(<CampaignPerformance campaignId={5} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Failed to load performance");
  });

  it("shows an empty state instead of bare axes when the campaign has no channels", () => {
    state.data = { totals: { links: 0, engagements: 0 }, channels: [] };
    const { container } = render(<CampaignPerformance campaignId={5} />);
    expect(screen.getByRole("heading", { name: "Add a channel to see performance" })).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(container.querySelector(".recharts-wrapper")).toBeNull();
  });
});
