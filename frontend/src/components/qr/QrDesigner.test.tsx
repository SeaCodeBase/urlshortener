import { describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { DEFAULT_QR_DESIGN, type QrDesign } from "@/types";
import { QrDesigner } from "./QrDesigner";

const upload = vi.hoisted(() => ({ mutateAsync: vi.fn(), isPending: false }));
vi.mock("@/lib/hooks", () => ({ useUploadImage: () => upload }));

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

  it("applies an uploaded logo to the latest design, not the one from before the upload", async () => {
    let resolve!: (v: { url: string }) => void;
    upload.mutateAsync.mockReturnValueOnce(new Promise((r) => (resolve = r)));
    const onChange = vi.fn();
    const { rerender } = wrap(<QrDesigner design={DEFAULT_QR_DESIGN} onChange={onChange} data="x" />);
    const file = new File(["png"], "logo.png", { type: "image/png" });
    fireEvent.change(screen.getByLabelText("Logo image (PNG or JPEG)"), { target: { files: [file] } });
    const edited: QrDesign = { ...DEFAULT_QR_DESIGN, pattern: "circle" };
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <QrDesigner design={edited} onChange={onChange} data="x" />
      </QueryClientProvider>,
    );
    await act(async () => resolve({ url: "/uploads/logo.png" }));
    expect(onChange).toHaveBeenLastCalledWith({ ...edited, logo: "/uploads/logo.png" });
  });

  it("Enter in Frame text does not submit an enclosing form", () => {
    const onSubmit = vi.fn((e: { preventDefault: () => void }) => e.preventDefault());
    wrap(
      <form onSubmit={onSubmit}>
        <QrDesigner design={{ ...DEFAULT_QR_DESIGN, frame: "text-bottom" }} onChange={vi.fn()} data="x" />
        <button type="submit">Create</button>
      </form>,
    );
    const input = screen.getByLabelText("Frame text");
    const event = fireEvent.keyDown(input, { key: "Enter" });
    expect(event).toBe(false); // default prevented, so no implicit submission
  });
});
