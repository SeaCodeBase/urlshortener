import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Webhook } from "@/types";
import WebhooksPage from "./page";

const m = {
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
  verify: vi.fn(),
  deactivate: vi.fn(),
  activate: vi.fn(),
};
const toastSuccess = vi.fn();
const toastError = vi.fn();
const state: { list: Webhook[] | undefined; isLoading: boolean; isError: boolean; deliveryCalls: unknown[][] } = {
  list: [],
  isLoading: false,
  isError: false,
  deliveryCalls: [],
};

vi.mock("sonner", () => ({
  toast: { success: (s: string) => toastSuccess(s), error: (s: string) => toastError(s) },
}));
vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: unknown) => unknown) => sel({ groups: [{ id: 3, name: "Marketing" }], activeGroupId: 3 }),
}));
vi.mock("@/lib/hooks", () => ({
  useWebhooks: () => ({ data: state.list, isLoading: state.isLoading, isError: state.isError }),
  useWebhookDeliveries: (...args: unknown[]) => {
    state.deliveryCalls.push(args);
    return { data: [], isLoading: false, isError: false };
  },
  useWebhookMutations: () =>
    Object.fromEntries(Object.entries(m).map(([k, fn]) => [k, { mutateAsync: fn, isPending: false }])),
}));

function hook(id: number, name: string, status: Webhook["status"], last: string | null = null): Webhook {
  return {
    id,
    user_id: 1,
    group_id: 3,
    name,
    url: `https://example.com/${id}`,
    event: "engagement",
    secret: "x",
    is_active: status !== "deactivated",
    status,
    consecutive_failures: 0,
    failing_since: null,
    last_delivery_at: last,
    created_at: "",
    updated_at: "",
  };
}

describe("WebhooksPage", () => {
  beforeEach(() => {
    Object.values(m).forEach((f) => f.mockReset().mockResolvedValue({}));
    toastSuccess.mockReset();
    toastError.mockReset();
    state.list = [hook(1, "Zapier", "active", "2026-10-24T09:37:00Z"), hook(2, "Slack", "deactivated"), hook(3, "CRM", "alert")];
    state.isLoading = false;
    state.isError = false;
    state.deliveryCalls = [];
  });

  it("shows the empty state", () => {
    state.list = [];
    render(<WebhooksPage />);
    expect(screen.getByText("No webhooks yet")).toBeInTheDocument();
    expect(
      screen.getByText("Get notified with a signed POST each time one of your links is engaged."),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Create webhook" })).toHaveLength(2);
  });

  it("shows the load error", () => {
    state.list = undefined;
    state.isError = true;
    render(<WebhooksPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("Failed to load webhooks");
  });

  it("renders status chips and last delivery", () => {
    render(<WebhooksPage />);
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText("Disabled")).toBeInTheDocument();
    expect(screen.getByText("Alert")).toBeInTheDocument();
    expect(screen.getByText("October 24, 2026 9:37 AM UTC")).toBeInTheDocument();
    expect(screen.getAllByText("—")).toHaveLength(2);
  });

  it("verifies and toasts the status code", async () => {
    m.verify.mockResolvedValue({ delivery: { status_code: 204, error: "" }, webhook: state.list![0] });
    const user = userEvent.setup();
    render(<WebhooksPage />);
    await user.click(screen.getByRole("button", { name: "More actions for Zapier" }));
    await user.click(await screen.findByRole("menuitem", { name: "Verify" }));
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Ping delivered (204)"));
    expect(m.verify).toHaveBeenCalledWith(1);
  });

  it("toasts the delivery error when the ping fails", async () => {
    m.verify.mockResolvedValue({ delivery: { status_code: 500, error: "" }, webhook: state.list![0] });
    const user = userEvent.setup();
    render(<WebhooksPage />);
    await user.click(screen.getByRole("button", { name: "More actions for Zapier" }));
    await user.click(await screen.findByRole("menuitem", { name: "Verify" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("Ping failed (500)"));
  });

  it("deactivates an active webhook and activates a disabled one", async () => {
    const user = userEvent.setup();
    render(<WebhooksPage />);
    await user.click(screen.getByRole("button", { name: "More actions for Zapier" }));
    await user.click(await screen.findByRole("menuitem", { name: "Deactivate" }));
    await waitFor(() => expect(m.deactivate).toHaveBeenCalledWith(1));

    await user.click(screen.getByRole("button", { name: "More actions for Slack" }));
    expect(screen.queryByRole("menuitem", { name: "Deactivate" })).not.toBeInTheDocument();
    await user.click(await screen.findByRole("menuitem", { name: "Activate" }));
    await waitFor(() => expect(m.activate).toHaveBeenCalledWith(2));
  });

  it("deletes after confirming", async () => {
    const user = userEvent.setup();
    render(<WebhooksPage />);
    await user.click(screen.getByRole("button", { name: "More actions for CRM" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Delete CRM?")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(m.remove).toHaveBeenCalledWith(3));
  });

  it("opens the edit dialog prefilled", async () => {
    const user = userEvent.setup();
    render(<WebhooksPage />);
    await user.click(screen.getByRole("button", { name: "More actions for Slack" }));
    await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Name")).toHaveValue("Slack");
  });

  it("toggles one deliveries panel at a time when a row is clicked", async () => {
    const user = userEvent.setup();
    render(<WebhooksPage />);
    await user.click(screen.getByText("Zapier"));
    expect(screen.getByText("No deliveries yet")).toBeInTheDocument();
    expect(state.deliveryCalls.at(-1)).toEqual([1, true]);

    await user.click(screen.getByText("Slack"));
    expect(screen.getAllByText("No deliveries yet")).toHaveLength(1);
    expect(state.deliveryCalls.at(-1)).toEqual([2, true]);

    await user.click(screen.getByText("Slack"));
    expect(screen.queryByText("No deliveries yet")).not.toBeInTheDocument();
  });
});
