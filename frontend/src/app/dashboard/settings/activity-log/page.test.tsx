import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ActivityLogPage from "./page";

const replace = vi.fn();
const nav = { params: new URLSearchParams() };
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => "/dashboard/settings/activity-log",
  useSearchParams: () => nav.params,
}));
vi.mock("@/lib/hooks", () => ({
  useActivity: () => ({ data: { entries: [], total_pages: 1 }, isLoading: false }),
}));

beforeEach(() => {
  replace.mockReset();
  nav.params = new URLSearchParams();
});

describe("Activity log — empty", () => {
  it("explains what will appear", () => {
    render(<ActivityLogPage />);
    expect(screen.getByRole("heading", { name: "No activity yet" })).toBeInTheDocument();
    expect(
      screen.getByText("Changes to your links, QR Codes, domains and settings will appear here."),
    ).toBeInTheDocument();
  });

  it("offers to clear an active date filter", async () => {
    nav.params = new URLSearchParams("from=2026-10-01&to=2026-10-02");
    render(<ActivityLogPage />);
    expect(screen.getByRole("heading", { name: "No activity in this date range" })).toBeInTheDocument();
    expect(screen.getByText("Try a different date range.")).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Clear date filter" }));
    expect(replace).toHaveBeenCalledWith("/dashboard/settings/activity-log");
  });
});
