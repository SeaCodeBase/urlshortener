import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiError } from "@/lib/api";
import type { CampaignDetail, CampaignSummary, Channel } from "@/types";
import { CampaignDialog } from "./CampaignDialog";

const create = vi.fn();
const update = vi.fn();
const createChannel = vi.fn();
const toastSuccess = vi.fn();
const state: { channels: Channel[]; detail: CampaignDetail | undefined; detailId: unknown } = {
  channels: [],
  detail: undefined,
  detailId: undefined,
};

vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: vi.fn() } }));
vi.mock("@/lib/hooks", () => ({
  useChannels: () => ({ data: state.channels, isPending: false }),
  useCampaign: (id: unknown) => {
    state.detailId = id;
    return { data: id ? state.detail : undefined, isPending: !!id && !state.detail };
  },
  useCampaignMutations: () => ({
    create: { mutateAsync: create, isPending: false },
    update: { mutateAsync: update, isPending: false },
    createChannel: { mutateAsync: createChannel, isPending: false },
  }),
}));

function channel(id: number, name: string): Channel {
  return { id, group_id: 1, name, created_at: "" };
}

const base = {
  id: 5,
  user_id: 1,
  group_id: 1,
  name: "Fall launch",
  description: "Q4 push",
  auto_utm: true,
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
};

describe("CampaignDialog", () => {
  beforeEach(() => {
    create.mockReset().mockResolvedValue({ ...base, channels: [] });
    update.mockReset().mockResolvedValue({ ...base, channels: [] });
    createChannel.mockReset();
    toastSuccess.mockReset();
    state.channels = [channel(1, "Email"), channel(2, "Social")];
    state.detail = undefined;
    state.detailId = undefined;
  });

  it("creates a campaign with the full payload shape", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<CampaignDialog open onOpenChange={onOpenChange} />);
    expect(screen.getByRole("heading", { name: "Create campaign" })).toBeInTheDocument();
    expect(
      screen.getByText(
        "Adds utm_medium=bitly, utm_source={channel} and utm_campaign={name} to links created from a channel",
      ),
    ).toBeInTheDocument();

    const submit = screen.getByRole("button", { name: "Create campaign" });
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText("Name"), " Spring sale ");
    await user.type(screen.getByLabelText("Description"), "Big one");
    await user.click(screen.getByRole("checkbox", { name: "Social" }));
    await user.click(screen.getByRole("checkbox", { name: "Auto-add UTM parameters" }));
    await user.click(submit);

    expect(create).toHaveBeenCalledWith({
      name: "Spring sale",
      description: "Big one",
      auto_utm: true,
      channel_ids: [2],
      new_channels: [],
    });
    expect(toastSuccess).toHaveBeenCalledWith("Campaign created");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("edits a campaign detail, preselecting its channels and sending the full set", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    const detail: CampaignDetail = { ...base, channels: [{ ...channel(1, "Email"), links: [] }] };
    render(<CampaignDialog open onOpenChange={onOpenChange} campaign={detail} />);
    expect(screen.getByRole("heading", { name: "Edit campaign" })).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Fall launch");
    expect(screen.getByRole("checkbox", { name: "Email" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("checkbox", { name: "Auto-add UTM parameters" })).toHaveAttribute("aria-checked", "true");
    await user.click(screen.getByRole("checkbox", { name: "Social" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(update).toHaveBeenCalledWith({
      id: 5,
      input: { name: "Fall launch", description: "Q4 push", auto_utm: true, channel_ids: [1, 2], new_channels: [] },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Campaign updated");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("fetches the detail for a summary to preselect channels", async () => {
    const summary: CampaignSummary = { ...base, channel_count: 1, link_count: 0, engagements: 0 };
    state.detail = { ...base, channels: [{ ...channel(2, "Social"), links: [] }] };
    render(<CampaignDialog open onOpenChange={vi.fn()} campaign={summary} />);
    expect(state.detailId).toBe(5);
    expect(await screen.findByRole("checkbox", { name: "Social" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("checkbox", { name: "Email" })).toHaveAttribute("aria-checked", "false");
  });

  it("shows the API error inline and stays open", async () => {
    create.mockRejectedValue(new ApiError("name is too long", 400));
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<CampaignDialog open onOpenChange={onOpenChange} />);
    await user.type(screen.getByLabelText("Name"), "X");
    await user.click(screen.getByRole("button", { name: "Create campaign" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("name is too long");
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
