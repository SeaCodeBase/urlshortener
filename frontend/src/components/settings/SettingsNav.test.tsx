import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { SettingsNav } from "./SettingsNav";

let pathname = "/dashboard/settings/profile";

vi.mock("next/navigation", () => ({
  usePathname: () => pathname,
}));

describe("SettingsNav", () => {
  beforeEach(() => {
    pathname = "/dashboard/settings/profile";
  });

  it("renders the three group headings and the items in order", () => {
    render(<SettingsNav />);
    const nav = screen.getByRole("navigation", { name: "Settings" });
    expect(within(nav).getByText("Personal settings")).toBeInTheDocument();
    expect(within(nav).getByText("Account settings")).toBeInTheDocument();
    expect(within(nav).getByText("Developer settings")).toBeInTheDocument();
    const links = within(nav).getAllByRole("link");
    expect(links.map((a) => a.textContent?.trim())).toEqual([
      "Profile",
      "Account details",
      "Groups",
      "Webhooks",
      "Bulk upload",
      "Activity log",
      "API",
    ]);
    expect(links.map((a) => a.getAttribute("href"))).toEqual([
      "/dashboard/settings/profile",
      "/dashboard/settings/account",
      "/dashboard/settings/groups",
      "/dashboard/settings/webhooks",
      "/dashboard/settings/bulk-upload",
      "/dashboard/settings/activity-log",
      "/dashboard/settings/api",
    ]);
  });

  it("marks Groups active on a group detail route", () => {
    pathname = "/dashboard/settings/groups/5";
    render(<SettingsNav />);
    expect(screen.getByRole("link", { name: "Groups" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Profile" })).not.toHaveAttribute("aria-current");
  });
});
