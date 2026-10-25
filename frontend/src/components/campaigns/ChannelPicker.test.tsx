import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import type { Channel } from "@/types";
import { ChannelPicker } from "./ChannelPicker";

const createChannel = vi.fn();
const state: { channels: Channel[] } = { channels: [] };

vi.mock("@/lib/hooks", () => ({
  useChannels: () => ({ data: state.channels, isPending: false }),
  useCampaignMutations: () => ({ createChannel: { mutateAsync: createChannel, isPending: false } }),
}));

function channel(id: number, name: string): Channel {
  return { id, group_id: 1, name, created_at: "" };
}

function Harness({ initial = [], onChange }: { initial?: number[]; onChange: (ids: number[]) => void }) {
  const [value, setValue] = useState<number[]>(initial);
  return (
    <ChannelPicker
      value={value}
      onChange={(ids) => {
        setValue(ids);
        onChange(ids);
      }}
    />
  );
}

describe("ChannelPicker", () => {
  beforeEach(() => {
    createChannel.mockReset();
    state.channels = [channel(1, "Email"), channel(2, "Social")];
  });

  it("lists channels as checkboxes and toggles selection", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={[1]} onChange={onChange} />);
    expect(screen.getByRole("checkbox", { name: "Email" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("checkbox", { name: "Social" })).toHaveAttribute("aria-checked", "false");
    await user.click(screen.getByRole("checkbox", { name: "Social" }));
    expect(onChange).toHaveBeenLastCalledWith([1, 2]);
    await user.click(screen.getByRole("checkbox", { name: "Email" }));
    expect(onChange).toHaveBeenLastCalledWith([2]);
  });

  it("creates a new channel and selects it", async () => {
    createChannel.mockResolvedValue(channel(7, "Podcast"));
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={[1]} onChange={onChange} />);
    const input = screen.getByLabelText("New channel name");
    await user.type(input, "  Podcast ");
    await user.click(screen.getByRole("button", { name: "Create new channel" }));
    expect(createChannel).toHaveBeenCalledWith("Podcast");
    expect(onChange).toHaveBeenLastCalledWith([1, 7]);
    expect(input).toHaveValue("");
  });

  it("selects an existing channel instead of creating a duplicate", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness onChange={onChange} />);
    await user.type(screen.getByLabelText("New channel name"), "social{Enter}");
    expect(createChannel).not.toHaveBeenCalled();
    expect(onChange).toHaveBeenLastCalledWith([2]);
  });

  it("shows the API error inline when creating fails", async () => {
    createChannel.mockRejectedValue(new Error("Channel already exists"));
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness onChange={onChange} />);
    await user.type(screen.getByLabelText("New channel name"), "Ads");
    await user.click(screen.getByRole("button", { name: "Create new channel" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Channel already exists");
    expect(onChange).not.toHaveBeenCalled();
  });
});
