import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiError } from "@/lib/api";
import type { CampaignDetail, CampaignLinkItem, ChannelDetail, Link } from "@/types";
import { AddLinkToChannelDialog } from "./AddLinkToChannelDialog";

const addLinks = vi.fn();
const createLinkInChannel = vi.fn();
const toastSuccess = vi.fn();
const state: { links: Link[]; params: unknown } = { links: [], params: undefined };

vi.mock("sonner", () => ({ toast: { success: (m: string) => toastSuccess(m), error: vi.fn() } }));
vi.mock("@/components/common/Favicon", () => ({ Favicon: () => <span data-testid="favicon" /> }));
vi.mock("@/lib/hooks", () => ({
  useDebouncedValue: <T,>(v: T) => v,
  useLinks: (params: unknown) => {
    state.params = params;
    return { data: { pages: [{ links: state.links, total: state.links.length, page: 1, total_pages: 1 }] }, isLoading: false, isError: false };
  },
  useCampaignMutations: () => ({
    addLinks: { mutateAsync: addLinks, isPending: false },
    createLinkInChannel: { mutateAsync: createLinkInChannel, isPending: false },
  }),
}));

function link(id: number, title: string): Link {
  return {
    id,
    user_id: 1,
    group_id: 1,
    short_code: `c${id}`,
    original_url: `https://example.com/${id}`,
    title,
    expires_at: null,
    is_active: true,
    archived: false,
    has_custom_back_half: false,
    redirect_count: 0,
    source: "link",
    created_at: "2026-10-20T00:00:00Z",
    updated_at: "2026-10-20T00:00:00Z",
    short_url: `http://localhost:8080/c${id}`,
    tags: [],
    total_clicks: 0,
    favicon_url: "",
  };
}

const member: CampaignLinkItem = { ...link(1, "Already in"), channel_id: 10, channel_name: "Email" };
const channel: ChannelDetail = { id: 10, group_id: 1, name: "Email", created_at: "", links: [member] };
const campaign: CampaignDetail = {
  id: 5,
  user_id: 1,
  group_id: 1,
  name: "Fall launch",
  description: "",
  auto_utm: false,
  created_at: "",
  updated_at: "",
  channels: [channel],
};

beforeEach(() => {
  addLinks.mockReset().mockResolvedValue(undefined);
  createLinkInChannel.mockReset().mockResolvedValue(link(9, "New"));
  toastSuccess.mockReset();
  state.links = [link(1, "Already in"), link(2, "Second"), link(3, "Third")];
  state.params = undefined;
});

describe("AddLinkToChannelDialog", () => {
  it("adds the picked links to the channel, excluding links already in it", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<AddLinkToChannelDialog open onOpenChange={onOpenChange} campaign={campaign} channel={channel} />);
    expect(state.params).toEqual({ q: "", limit: 20 });
    expect(screen.queryByRole("checkbox", { name: "Already in" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add links" })).toBeDisabled();

    await user.type(screen.getByRole("searchbox", { name: "Search links" }), "th");
    expect(state.params).toEqual({ q: "th", limit: 20 });

    await user.click(screen.getByRole("checkbox", { name: "Second" }));
    await user.click(screen.getByRole("checkbox", { name: "Third" }));
    await user.click(screen.getByRole("button", { name: "Add 2 links" }));

    expect(addLinks).toHaveBeenCalledWith({ campaignId: 5, channelId: 10, linkIds: [2, 3] });
    expect(toastSuccess).toHaveBeenCalledWith("Added to Email");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("creates a link in the channel with the optional fields", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<AddLinkToChannelDialog open onOpenChange={onOpenChange} campaign={campaign} channel={channel} />);
    await user.click(screen.getByRole("tab", { name: "Create link" }));
    expect(screen.queryByText(/UTM parameters will be added/)).not.toBeInTheDocument();

    await user.type(screen.getByLabelText("Destination URL"), "https://shop.example/fall");
    await user.type(screen.getByLabelText("Title (optional)"), " Fall shop ");
    await user.type(screen.getByLabelText("Back-half (optional)"), "fall-26");
    await user.click(screen.getByRole("button", { name: "Create link" }));

    expect(createLinkInChannel).toHaveBeenCalledWith({
      campaignId: 5,
      channelId: 10,
      input: { original_url: "https://shop.example/fall", title: "Fall shop", custom_code: "fall-26" },
    });
    expect(toastSuccess).toHaveBeenCalledWith("Link created");
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("sends only the destination when the optional fields are empty", async () => {
    const user = userEvent.setup();
    render(<AddLinkToChannelDialog open onOpenChange={vi.fn()} campaign={campaign} channel={channel} />);
    await user.click(screen.getByRole("tab", { name: "Create link" }));
    const submit = screen.getByRole("button", { name: "Create link" });
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText("Destination URL"), "https://shop.example/");
    await user.click(submit);
    expect(createLinkInChannel).toHaveBeenCalledWith({
      campaignId: 5,
      channelId: 10,
      input: { original_url: "https://shop.example/" },
    });
  });

  it("validates the back-half", async () => {
    const user = userEvent.setup();
    render(<AddLinkToChannelDialog open onOpenChange={vi.fn()} campaign={campaign} channel={channel} />);
    await user.click(screen.getByRole("tab", { name: "Create link" }));
    await user.type(screen.getByLabelText("Destination URL"), "https://shop.example/");
    await user.type(screen.getByLabelText("Back-half (optional)"), "no spaces");
    expect(screen.getByText("Only letters, numbers, - and _ are allowed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create link" })).toBeDisabled();
  });

  it("shows the UTM note when the campaign adds UTM parameters", async () => {
    const user = userEvent.setup();
    render(
      <AddLinkToChannelDialog open onOpenChange={vi.fn()} campaign={{ ...campaign, auto_utm: true }} channel={channel} />,
    );
    await user.click(screen.getByRole("tab", { name: "Create link" }));
    expect(
      screen.getByText("UTM parameters will be added: utm_medium=bitly, utm_source=Email, utm_campaign=Fall launch"),
    ).toBeInTheDocument();
  });

  it("shows API errors inline and stays open", async () => {
    createLinkInChannel.mockRejectedValue(new ApiError("That back-half is already taken", 409));
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(<AddLinkToChannelDialog open onOpenChange={onOpenChange} campaign={campaign} channel={channel} />);
    await user.click(screen.getByRole("tab", { name: "Create link" }));
    await user.type(screen.getByLabelText("Destination URL"), "https://shop.example/");
    await user.click(screen.getByRole("button", { name: "Create link" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("That back-half is already taken");
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
