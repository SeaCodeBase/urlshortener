import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Link, QrCode } from "@/types";
import { DashboardFilterDialog } from "./DashboardFilterDialog";

let links: Partial<Link>[] = [];
const qrs: Partial<QrCode>[] = [{ id: 5, title: "Menu", short_url: "http://s.test/m" }];

vi.mock("@/lib/hooks", () => ({
  useLinks: () => ({
    data: { pages: [{ links, total: links.length, page: 1, total_pages: 1 }] },
    isLoading: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  }),
  useQrCodes: () => ({
    data: { pages: [{ qr_codes: qrs, total: qrs.length, page: 1, total_pages: 1 }] },
    isLoading: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  }),
  useMeta: () => ({ data: undefined }),
}));

function makeLinks(n: number): Partial<Link>[] {
  return Array.from({ length: n }, (_, i) => ({
    id: i + 1,
    title: `Link ${i + 1}`,
    short_url: `http://s.test/l${i + 1}`,
    original_url: "https://example.com",
  }));
}

describe("DashboardFilterDialog", () => {
  beforeEach(() => {
    links = makeLinks(3);
  });

  it("applies the selected links", async () => {
    const onApply = vi.fn().mockResolvedValue(undefined);
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<DashboardFilterDialog open onOpenChange={onOpenChange} value={{ type: "all" }} onApply={onApply} />);

    expect(screen.getByRole("heading", { name: "Filter" })).toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: "Links" }));
    await user.click(screen.getByRole("checkbox", { name: "Link 1" }));
    await user.click(screen.getByRole("checkbox", { name: "Link 3" }));
    expect(screen.getByText("2/200 selected")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Apply filters" }));

    expect(onApply).toHaveBeenCalledWith({ type: "links", ids: [1, 3] });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables unchecked boxes once 200 are selected", async () => {
    links = makeLinks(201);
    const ids = links.slice(0, 200).map((l) => l.id!);
    render(<DashboardFilterDialog open onOpenChange={vi.fn()} value={{ type: "links", ids }} onApply={vi.fn()} />);

    expect(screen.getByText("200/200 selected")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Link 201" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "Link 200" })).toBeEnabled();
  });

  it("All resets the filter", async () => {
    const onApply = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(<DashboardFilterDialog open onOpenChange={vi.fn()} value={{ type: "qr", ids: [5] }} onApply={onApply} />);

    expect(screen.getByRole("checkbox", { name: "Menu" })).toBeChecked();
    await user.click(screen.getByRole("radio", { name: "All" }));
    await user.click(screen.getByRole("button", { name: "Apply filters" }));
    expect(onApply).toHaveBeenCalledWith({ type: "all" });
  });
});
