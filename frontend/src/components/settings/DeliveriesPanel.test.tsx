import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { WebhookDelivery } from "@/types";
import { DeliveriesPanel } from "./DeliveriesPanel";

const state: { data: WebhookDelivery[] | undefined; calls: unknown[][] } = { data: [], calls: [] };

vi.mock("@/lib/hooks", () => ({
  useWebhookDeliveries: (...args: unknown[]) => {
    state.calls.push(args);
    return { data: state.data, isLoading: false, isError: false };
  },
}));

function delivery(id: number, status_code: number, attempt: number, error = ""): WebhookDelivery {
  return { id, webhook_id: 4, event_id: `e${id}`, status_code, attempt, delivered_at: "2026-10-24T09:37:00Z", error };
}

describe("DeliveriesPanel", () => {
  beforeEach(() => {
    state.data = [];
    state.calls = [];
  });

  it("renders the empty text", () => {
    render(<DeliveriesPanel webhookId={4} open />);
    expect(screen.getByText("No deliveries yet")).toBeInTheDocument();
    expect(state.calls[0]).toEqual([4, true]);
  });

  it("renders one row per delivery with code, attempt, time and error", () => {
    state.data = [delivery(1, 200, 1), delivery(2, 0, 3, "connection refused"), delivery(3, 500, 2, "boom")];
    render(<DeliveriesPanel webhookId={4} open />);
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    expect(screen.getByText("200")).toBeInTheDocument();
    expect(screen.getByText("0")).toBeInTheDocument();
    expect(screen.getByText("Attempt 3")).toBeInTheDocument();
    expect(screen.getByText("connection refused")).toBeInTheDocument();
    expect(screen.getAllByText("October 24, 2026 9:37 AM UTC")).toHaveLength(3);
  });
});
