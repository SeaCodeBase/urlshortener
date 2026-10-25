import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import ApiSettingsPage from "./page";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/settings/TokenCreateDialog", () => ({ TokenCreateDialog: () => null }));
vi.mock("@/lib/hooks", () => ({
  useTokens: () => ({ data: [], isLoading: false }),
  useRevokeToken: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

describe("API settings — no tokens", () => {
  it("shows the empty state with a next step", () => {
    render(<ApiSettingsPage />);
    expect(screen.getByRole("heading", { name: "No tokens generated yet" })).toBeInTheDocument();
    expect(screen.getByText("Generate a token to connect other apps or call the API.")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Generate token" })).toHaveLength(1);
  });
});
