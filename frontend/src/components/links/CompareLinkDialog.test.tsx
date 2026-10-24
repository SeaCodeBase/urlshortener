import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { Link } from "@/types";
import { CompareLinkDialog } from "./CompareLinkDialog";

const link = (id: number, title: string): Link =>
  ({ id, title, short_url: `https://sho.rt/l${id}`, original_url: `https://example.com/${id}` }) as Link;

const useLinks = vi.fn();
const fetchNextPage = vi.fn();
vi.mock("@/lib/hooks", () => ({
  useLinks: (params: unknown) => useLinks(params),
}));

describe("CompareLinkDialog", () => {
  beforeEach(() => {
    useLinks.mockReset();
    fetchNextPage.mockReset();
    useLinks.mockReturnValue({
      data: { pages: [{ links: [link(7, "Current"), link(8, "Spring sale"), link(9, "")], page: 1, total_pages: 1, total: 3 }] },
      isLoading: false,
      hasNextPage: false,
      fetchNextPage,
    });
  });

  it("lists the group's links except the current one and returns the chosen id", () => {
    const onCompare = vi.fn();
    render(<CompareLinkDialog open onOpenChange={() => {}} currentId={7} onCompare={onCompare} />);

    expect(screen.getByRole("heading", { name: "Compare to a link" })).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Search links")).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: /Current/ })).not.toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2);
    // A link without a title falls back to its short URL.
    expect(screen.getByRole("radio", { name: /sho\.rt\/l9/ })).toBeInTheDocument();

    const compare = screen.getByRole("button", { name: "Compare" });
    expect(compare).toBeDisabled();
    fireEvent.click(screen.getByRole("radio", { name: /Spring sale/ }));
    expect(compare).toBeEnabled();
    fireEvent.click(compare);
    expect(onCompare).toHaveBeenCalledWith(8);
    expect(useLinks.mock.calls[0][0]).toMatchObject({ limit: 50 });
  });

  it("searches by q and loads more pages", () => {
    useLinks.mockReturnValue({
      data: { pages: [{ links: [link(8, "Spring sale")], page: 1, total_pages: 2, total: 51 }] },
      isLoading: false,
      hasNextPage: true,
      fetchNextPage,
    });
    vi.useFakeTimers();
    try {
      render(<CompareLinkDialog open onOpenChange={() => {}} currentId={7} onCompare={() => {}} />);
      fireEvent.change(screen.getByPlaceholderText("Search links"), { target: { value: "spring" } });
      act(() => vi.advanceTimersByTime(400));
    } finally {
      vi.useRealTimers();
    }
    expect(useLinks.mock.calls.at(-1)?.[0]).toMatchObject({ q: "spring", limit: 50 });
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(fetchNextPage).toHaveBeenCalledTimes(1);
  });
});
