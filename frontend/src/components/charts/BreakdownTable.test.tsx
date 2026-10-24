import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { BreakdownTable, type BreakdownRow } from "./BreakdownTable";

const rows: BreakdownRow[] = Array.from({ length: 23 }, (_, i) => ({
  key: `r${i + 1}`,
  label: `Referrer ${i + 1}`,
  clicks: 100 - i,
  percentage: i === 0 ? 38 : 2,
  prev_clicks: 0,
}));

function bodyRows() {
  const [, body] = screen.getAllByRole("rowgroup");
  return within(body).getAllByRole("row");
}

describe("BreakdownTable", () => {
  it("shows the header and the percentage as a whole number", () => {
    render(<BreakdownTable labelHeader="Referrer" rows={rows} />);
    expect(screen.getByRole("columnheader", { name: "Referrer" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Engagements" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "%" })).toBeInTheDocument();
    expect(screen.getByText("38%")).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "vs prev." })).not.toBeInTheDocument();
  });

  it("paginates 10 rows per page with next / previous", () => {
    render(<BreakdownTable labelHeader="Referrer" rows={rows} />);
    expect(bodyRows()).toHaveLength(10);
    expect(screen.getByText("Referrer 1")).toBeInTheDocument();
    expect(screen.queryByText("Referrer 11")).not.toBeInTheDocument();
    expect(screen.getByText("1–10 of 23")).toBeInTheDocument();
    const prev = screen.getByRole("button", { name: "Previous page" });
    const next = screen.getByRole("button", { name: "Next page" });
    expect(prev).toBeDisabled();

    fireEvent.click(next);
    expect(screen.getByText("Referrer 11")).toBeInTheDocument();
    expect(screen.queryByText("Referrer 1")).not.toBeInTheDocument();
    expect(screen.getByText("11–20 of 23")).toBeInTheDocument();

    fireEvent.click(next);
    expect(bodyRows()).toHaveLength(3);
    expect(next).toBeDisabled();

    fireEvent.click(prev);
    expect(screen.getByText("Referrer 11")).toBeInTheDocument();
  });

  it("hides pagination for 10 rows or fewer", () => {
    render(<BreakdownTable labelHeader="Referrer" rows={rows.slice(0, 10)} />);
    expect(screen.queryByRole("button", { name: "Next page" })).not.toBeInTheDocument();
  });

  it("shows vs prev. change when comparing", () => {
    render(
      <BreakdownTable
        labelHeader="Country"
        compare
        rows={[{ key: "US", label: "United States", clicks: 30, percentage: 60, prev_clicks: 20 }]}
      />,
    );
    expect(screen.getByRole("columnheader", { name: "vs prev." })).toBeInTheDocument();
    expect(screen.getByText("+50.0%")).toBeInTheDocument();
  });

  it("shows the empty message without rows", () => {
    render(<BreakdownTable labelHeader="Referrer" rows={[]} />);
    expect(screen.getByText("No data for this period.")).toBeInTheDocument();
  });
});
