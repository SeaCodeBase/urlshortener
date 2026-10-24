import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Sidebar } from "./Sidebar";

const push = vi.fn();
let pathname = "/dashboard";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => pathname,
}));

function renderSidebar() {
  return render(
    <TooltipProvider>
      <Sidebar />
    </TooltipProvider>
  );
}

describe("Sidebar", () => {
  beforeEach(() => {
    push.mockReset();
    pathname = "/dashboard";
    window.localStorage.clear();
  });

  it("renders the nav items in order", () => {
    renderSidebar();
    const nav = screen.getByRole("navigation");
    const labels = within(nav)
      .getAllByRole("link")
      .map((a) => a.textContent?.trim());
    expect(labels).toEqual([
      "Home",
      "Links",
      "QR Codes",
      "Pages",
      "Analytics",
      "Campaigns",
      "Custom domains",
      "Settings",
    ]);
  });

  it("marks Links active on a link detail route", () => {
    pathname = "/dashboard/links/123";
    renderSidebar();
    const nav = screen.getByRole("navigation");
    expect(within(nav).getByRole("link", { name: "Links" })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: "Home" })).not.toHaveAttribute("aria-current");
  });

  it("collapse button toggles aria-expanded and persists the state", () => {
    renderSidebar();
    const toggle = screen.getByRole("button", { name: /collapse sidebar/i });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(toggle);
    expect(screen.getByRole("button", { name: /expand sidebar/i })).toHaveAttribute("aria-expanded", "false");
    expect(window.localStorage.getItem("sidebar-collapsed")).toBe("true");
  });

  it("keyboard shortcut l opens create link, ignored in inputs", () => {
    renderSidebar();
    fireEvent.keyDown(window, { key: "l" });
    expect(push).toHaveBeenCalledWith("/dashboard/links/create");
    push.mockReset();

    const input = document.createElement("input");
    document.body.appendChild(input);
    fireEvent.keyDown(input, { key: "q" });
    expect(push).not.toHaveBeenCalled();
    input.remove();

    fireEvent.keyDown(window, { key: "q", metaKey: true });
    expect(push).not.toHaveBeenCalled();
    fireEvent.keyDown(window, { key: "q" });
    expect(push).toHaveBeenCalledWith("/dashboard/qr-codes/create");
  });
});
