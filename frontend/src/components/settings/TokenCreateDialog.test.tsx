import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { TokenCreateDialog } from "./TokenCreateDialog";

const mutateAsync = vi.fn();

vi.mock("@/lib/hooks", () => ({
  useCreateToken: () => ({ mutateAsync, isPending: false }),
}));

describe("TokenCreateDialog", () => {
  beforeEach(() => {
    mutateAsync.mockReset();
    mutateAsync.mockResolvedValue({
      token: "us_abc",
      id: 1,
      name: "CI",
      prefix: "abc",
      created_at: "2026-10-24T00:00:00Z",
    });
  });

  it("shows the raw token once after generating", async () => {
    render(<TokenCreateDialog open onOpenChange={() => {}} />);
    fireEvent.change(screen.getByLabelText("Token name"), { target: { value: "CI" } });
    fireEvent.change(screen.getByLabelText("Confirm your password"), { target: { value: "secret123" } });
    fireEvent.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() => expect(screen.getByText("us_abc")).toBeInTheDocument());
    expect(mutateAsync).toHaveBeenCalledWith({ name: "CI", password: "secret123" });
    expect(
      screen.getByText("Make sure to copy your token now. You won't be able to see it again.")
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument();
  });
});
