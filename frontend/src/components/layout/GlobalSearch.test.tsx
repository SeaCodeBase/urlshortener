import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { GlobalSearch } from "./GlobalSearch";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/dashboard",
}));

function openAndType(text: string) {
  render(<GlobalSearch />);
  fireEvent.click(screen.getByRole("button", { name: /search/i }));
  const input = screen.getByRole("searchbox");
  fireEvent.change(input, { target: { value: text } });
  return input;
}

describe("GlobalSearch", () => {
  beforeEach(() => push.mockReset());

  it("shows where to search for the typed query", () => {
    openAndType("test");
    expect(screen.getByText('Search for "test" in:')).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Links" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "QR Codes" })).toBeInTheDocument();
  });

  it("Enter searches links", () => {
    const input = openAndType("test");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(push).toHaveBeenCalledWith("/dashboard/links?q=test");
  });

  it("QR Codes searches QR codes", () => {
    openAndType("a b");
    fireEvent.click(screen.getByRole("button", { name: "QR Codes" }));
    expect(push).toHaveBeenCalledWith("/dashboard/qr-codes?q=a%20b");
  });
});
