import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiError } from "@/lib/api";
import type { Webhook } from "@/types";
import { WebhookDialog } from "./WebhookDialog";

const create = vi.fn();
const update = vi.fn();
const toastSuccess = vi.fn();

vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: vi.fn() } }));
vi.mock("@/lib/hooks", () => ({
  useWebhookMutations: () => ({
    create: { mutateAsync: create, isPending: false },
    update: { mutateAsync: update, isPending: false },
  }),
}));
vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: unknown) => unknown) =>
    sel({ groups: [{ id: 3, name: "Marketing" }], activeGroupId: 3 }),
}));

const hook: Webhook = {
  id: 9,
  user_id: 1,
  group_id: 3,
  name: "Zapier",
  url: "https://hooks.example.com/in",
  event: "engagement",
  secret: "a".repeat(64),
  is_active: true,
  status: "active",
  consecutive_failures: 0,
  failing_since: null,
  last_delivery_at: null,
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
};

describe("WebhookDialog", () => {
  beforeEach(() => {
    create.mockReset().mockResolvedValue({ ...hook, secret: "s3cr3t-hex" });
    update.mockReset().mockResolvedValue(hook);
    toastSuccess.mockReset();
  });

  it("creates with {name, url, event} and then shows the one-time secret", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<WebhookDialog open onOpenChange={onOpenChange} />);

    expect(screen.getByLabelText("Group")).toHaveValue("Marketing");
    expect(screen.getByRole("combobox", { name: "Event" })).toHaveTextContent("engagement");
    await user.type(screen.getByLabelText("Name"), "  Zapier ");
    await user.type(screen.getByLabelText("URL"), "https://hooks.example.com/in");
    await user.click(screen.getByRole("button", { name: "Create webhook" }));

    await waitFor(() => expect(screen.getByLabelText("Secret")).toHaveValue("s3cr3t-hex"));
    expect(create).toHaveBeenCalledWith({ name: "Zapier", url: "https://hooks.example.com/in", event: "engagement" });
    expect(toastSuccess).toHaveBeenCalledWith("Webhook created");
    expect(
      screen.getByText("Use this to verify the X-Signature header (HMAC-SHA256 of the body)."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy" })).toBeInTheDocument();
    expect(onOpenChange).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Done" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("edits with {id, input}, keeps the event locked, and closes", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<WebhookDialog open onOpenChange={onOpenChange} webhook={hook} />);

    expect(screen.getByRole("combobox", { name: "Event" })).toBeDisabled();
    const name = screen.getByLabelText("Name");
    await user.clear(name);
    await user.type(name, "Renamed");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(update).toHaveBeenCalledWith({
      id: 9,
      input: { name: "Renamed", url: "https://hooks.example.com/in", event: "engagement" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Webhook updated");
    expect(screen.queryByLabelText("Secret")).not.toBeInTheDocument();
  });

  it("shows the API error inline", async () => {
    create.mockRejectedValue(new ApiError("URL must be a public http(s) address", 400));
    const user = userEvent.setup();
    render(<WebhookDialog open onOpenChange={() => {}} />);
    await user.type(screen.getByLabelText("Name"), "Local");
    await user.type(screen.getByLabelText("URL"), "http://127.0.0.1/x");
    await user.click(screen.getByRole("button", { name: "Create webhook" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("URL must be a public http(s) address");
  });
});
