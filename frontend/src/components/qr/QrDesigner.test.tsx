import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { DEFAULT_QR_DESIGN, type QrDesign } from "@/types";
import { QrDesigner } from "./QrDesigner";

vi.mock("@/components/qr/QrPreview", () => ({
  QrPreview: ({ data }: { data: string }) => <div data-testid="qr-preview">{data}</div>,
}));

function wrap(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("QrDesigner", () => {
  it("picking the Rounded pattern calls onChange with pattern 'rounded'", async () => {
    const onChange = vi.fn();
    wrap(<QrDesigner design={DEFAULT_QR_DESIGN} onChange={onChange} data="https://sho.rt/x" />);
    await userEvent.click(within(screen.getByRole("radiogroup", { name: "Pattern" })).getByRole("radio", { name: "Rounded" }));
    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_QR_DESIGN, pattern: "rounded" });
  });

  it("Reset to default calls onChange with the default design", async () => {
    const onChange = vi.fn();
    const design: QrDesign = { ...DEFAULT_QR_DESIGN, color: "#D93025", pattern: "circle" };
    wrap(<QrDesigner design={design} onChange={onChange} data="https://sho.rt/x" />);
    await userEvent.click(screen.getByRole("button", { name: "Reset to default" }));
    expect(onChange).toHaveBeenCalledWith(DEFAULT_QR_DESIGN);
  });

  it("renders the live preview with the data", () => {
    wrap(<QrDesigner design={DEFAULT_QR_DESIGN} onChange={vi.fn()} data="https://sho.rt/x" />);
    expect(screen.getByTestId("qr-preview")).toHaveTextContent("https://sho.rt/x");
  });
});
