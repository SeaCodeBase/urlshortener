import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { DashboardData } from "@/types";
import { ModuleCard, capChartRows } from "./ModuleCard";

const DATA: DashboardData = {
  from: "2026-10-18",
  to: "2026-10-24",
  unit: "day",
  compare: true,
  total: 120,
  prev_total: 100,
  change_pct: 20,
  top_day: { date: "2026-10-20", count: 40, prev_count: 12, change_pct: 12.5 },
  top_location: { code: "US", name: "United States", count: 55 },
  series: [{ date: "2026-10-18", clicks: 3 }],
  prev_series: [],
  devices: [{ key: "mobile", clicks: 70, percentage: 58.3, prev_clicks: 50 }],
  referrers: [],
  countries: [],
  cities: [],
  items: [],
};

describe("ModuleCard", () => {
  it("offers the module's allowed visualizations with the current one checked", async () => {
    const onChangeViz = vi.fn();
    const user = userEvent.setup();
    render(<ModuleCard moduleKey="devices" viz="doughnut" data={DATA} compare={false} unit="day" onChangeViz={onChangeViz} />);

    expect(screen.getByRole("heading", { name: "Engagements by device" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Engagements by device options" }));
    await user.click(screen.getByRole("menuitem", { name: "Change visualization" }));

    const options = await screen.findAllByRole("menuitemradio");
    expect(options.map((o) => o.textContent)).toEqual(["Doughnut chart", "Bar chart", "Table"]);
    expect(screen.getByRole("menuitemradio", { name: "Doughnut chart" })).toHaveAttribute("aria-checked", "true");

    screen.getByRole("menuitemradio", { name: "Table" }).focus();
    await user.keyboard("{Enter}");
    expect(onChangeViz).toHaveBeenCalledWith("table");
  });

  it("top_day scorecard shows No data when there is no top day", () => {
    render(
      <ModuleCard moduleKey="top_day" viz="scorecard" data={{ ...DATA, top_day: null }} compare unit="day" onChangeViz={vi.fn()} />,
    );
    expect(screen.getByText(/No data/)).toBeInTheDocument();
  });

  it("top_day scorecard shows the day and the change when comparing", () => {
    render(<ModuleCard moduleKey="top_day" viz="scorecard" data={DATA} compare unit="day" onChangeViz={vi.fn()} />);
    expect(screen.getByText("Oct 20, 2026")).toBeInTheDocument();
    expect(screen.getByText("vs previous period")).toBeInTheDocument();
    expect(screen.getByText("+12.5%")).toBeInTheDocument();
  });

  it("top_day hour buckets read as day and hour", () => {
    const hourly = { ...DATA, unit: "hour" as const, top_day: { ...DATA.top_day!, date: "2026-10-20T13:00" } };
    render(<ModuleCard moduleKey="top_day" viz="scorecard" data={hourly} compare={false} unit="hour" onChangeViz={vi.fn()} />);
    expect(screen.getByText("Oct 20, 13:00")).toBeInTheDocument();
    expect(screen.queryByText("vs previous period")).not.toBeInTheDocument();
  });

  it("renders the drag handle with an accessible name", () => {
    render(
      <ModuleCard
        moduleKey="devices"
        viz="table"
        data={DATA}
        compare={false}
        unit="day"
        onChangeViz={vi.fn()}
        dragHandleProps={{}}
      />,
    );
    expect(screen.getByRole("button", { name: "Reorder Engagements by device" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Download Engagements by device CSV" })).toBeInTheDocument();
  });

  it("doughnut legend shows the top 8 referrers and folds the rest into Other", () => {
    const referrers = Array.from({ length: 12 }, (_, i) => ({
      referrer: `https://site${i}.example/`,
      clicks: 100 - i,
      percentage: 5,
      prev_clicks: 1,
    }));
    render(
      <ModuleCard moduleKey="referrers" viz="doughnut" data={{ ...DATA, referrers }} compare={false} unit="day" onChangeViz={vi.fn()} />,
    );
    const items = within(screen.getByRole("list", { name: "Legend" })).getAllByRole("listitem");
    expect(items).toHaveLength(9);
    expect(within(items[0]).getByText("https://site0.example/")).toBeInTheDocument();
    expect(within(items[8]).getByText("Other")).toBeInTheDocument();
    // 92 + 91 + 90 + 89
    expect(within(items[8]).getByText("362")).toBeInTheDocument();
  });
});

describe("capChartRows", () => {
  const row = (i: number) => ({ key: `k${i}`, label: `L${i}`, clicks: 10 + i, percentage: 2, prev_clicks: i });

  it("keeps up to 8 rows unchanged", () => {
    const rows = Array.from({ length: 8 }, (_, i) => row(i));
    expect(capChartRows(rows, true)).toEqual(rows);
  });

  it("folds rows after the 8th into one Other row", () => {
    const rows = Array.from({ length: 11 }, (_, i) => row(i));
    const capped = capChartRows(rows, true);
    expect(capped).toHaveLength(9);
    expect(capped.slice(0, 8)).toEqual(rows.slice(0, 8));
    expect(capped[8]).toEqual({ key: "__other", label: "Other", clicks: 18 + 19 + 20, percentage: 6, prev_clicks: 8 + 9 + 10 });
  });

  it("Other has no prev when not comparing", () => {
    const capped = capChartRows(Array.from({ length: 9 }, (_, i) => row(i)), false);
    expect(capped[8]).toEqual({ key: "__other", label: "Other", clicks: 18, percentage: 2 });
  });
});
