import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { CampaignDetail, CampaignLinkItem, ChannelDetail, ChannelStats } from "@/types";
import { ChannelCard, channelMeta } from "./ChannelCard";

vi.mock("@/lib/hooks", () => ({
  useCampaignMutations: () => ({ removeLink: { mutateAsync: vi.fn(), isPending: false } }),
}));
vi.mock("@/components/common/Favicon", () => ({ Favicon: () => <span data-testid="favicon" /> }));
vi.mock("./AddLinkToChannelDialog", () => ({
  AddLinkToChannelDialog: () => null,
  linkLabel: (l: { title?: string | null; short_url: string }) => l.title || l.short_url,
}));

const link = (id: number): CampaignLinkItem =>
  ({
    id,
    title: `Link ${id}`,
    short_url: `https://s.test/${id}`,
    original_url: "https://example.com",
    favicon_url: null,
    total_clicks: 3,
    channel_id: 9,
    channel_name: "Email",
  }) as unknown as CampaignLinkItem;

const channel = { id: 9, name: "Email", links: [link(1), link(2)] } as unknown as ChannelDetail;
const campaign = { id: 1, name: "Spring", channels: [channel] } as unknown as CampaignDetail;
const stats = { channel_id: 9, name: "Email", links: 2, engagements: 45, series: [] } as ChannelStats;

describe("ChannelCard header", () => {
  it("shows link and engagement counts from stats", () => {
    render(<ChannelCard campaign={campaign} channel={channel} stats={stats} />);
    expect(screen.getByText("2 links · 45 engagements in range")).toBeInTheDocument();
  });

  it("falls back to the channel's own link count while stats load", () => {
    render(<ChannelCard campaign={campaign} channel={channel} />);
    expect(screen.getByText("2 links · — engagements in range")).toBeInTheDocument();
  });

  it("uses singular forms", () => {
    expect(channelMeta(1, { links: 1, engagements: 1 })).toBe("1 link · 1 engagement in range");
    expect(channelMeta(1, undefined)).toBe("1 link · — engagements in range");
  });
});
