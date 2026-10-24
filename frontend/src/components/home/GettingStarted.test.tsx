import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { GettingStarted } from "./GettingStarted";

const state = { pagesTotal: 0 };

vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: unknown) => unknown) =>
    sel({ user: { id: 1, email: "jose@example.com", display_name: "Jose", home_checklist_dismissed: false } }),
}));
vi.mock("@/lib/hooks", () => ({
  useLinks: () => ({ data: { pages: [{ links: [], total: 0 }] } }),
  useQrCodes: () => ({ data: { pages: [{ qr_codes: [], total: 0 }] } }),
  useUpdateMe: () => ({ mutate: vi.fn(), isPending: false }),
  usePages: () => ({ data: { pages: [], total: state.pagesTotal } }),
}));

describe("GettingStarted", () => {
  beforeEach(() => {
    state.pagesTotal = 0;
  });

  it("leaves the page step open with a Create a page link when there are no pages", () => {
    render(<GettingStarted />);
    expect(screen.getByRole("img", { name: "0% complete" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Create a page" })).toHaveAttribute("href", "/dashboard/pages/new");
  });

  it("checks the page step when a page exists", () => {
    state.pagesTotal = 1;
    render(<GettingStarted />);
    expect(screen.getByRole("img", { name: "25% complete" })).toBeInTheDocument();
    expect(screen.getByText("Create a page")).toHaveClass("line-through");
    expect(screen.queryByRole("link", { name: "Create a page" })).not.toBeInTheDocument();
  });
});
