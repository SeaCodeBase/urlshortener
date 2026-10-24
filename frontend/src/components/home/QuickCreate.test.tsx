import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QuickCreate } from "./QuickCreate";

const mutateAsync = vi.fn().mockResolvedValue({ id: 1, short_url: "http://localhost:8080/abc" });
const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
}));

vi.mock("@/lib/hooks", () => ({
  useCreateLink: () => ({ mutateAsync, isPending: false }),
  useDomains: () => ({ data: [] }),
  useMeta: () => ({ data: { base_url: "http://localhost:8080" } }),
}));

describe("QuickCreate", () => {
  it("submits on Enter and shows the link-ready dialog", async () => {
    const user = userEvent.setup();
    render(<QuickCreate />);
    await user.type(screen.getByPlaceholderText("Paste a long URL to shorten it..."), "https://example.com/long{Enter}");
    expect(await screen.findByText("Your link is ready!")).toBeInTheDocument();
    expect(screen.getByText("localhost:8080/abc")).toBeInTheDocument();
    expect(mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ original_url: "https://example.com/long" }));
    expect(screen.getByRole("button", { name: /Copy link/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /View link details/ })).toHaveAttribute("href", "/dashboard/links/1");
  });

  it("QR Code mode navigates to the QR create page", async () => {
    const user = userEvent.setup();
    render(<QuickCreate />);
    await user.click(screen.getByRole("radio", { name: "QR Code" }));
    await user.type(screen.getByPlaceholderText("Paste a long URL to shorten it..."), "https://example.com/q");
    await user.click(screen.getByRole("button", { name: /Create your code/ }));
    expect(push).toHaveBeenCalledWith("/dashboard/qr-codes/create?url=" + encodeURIComponent("https://example.com/q"));
  });
});
