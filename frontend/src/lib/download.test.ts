import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadBlob } from "./download";

describe("downloadBlob", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("clicks a temporary download link and revokes the URL a second later", () => {
    vi.useFakeTimers();
    const create = vi.fn(() => "blob:test");
    const revoke = vi.fn();
    Object.assign(URL, { createObjectURL: create, revokeObjectURL: revoke });
    const clicks: HTMLAnchorElement[] = [];
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      clicks.push(this);
    });

    const blob = new Blob(["a,b"], { type: "text/csv" });
    downloadBlob("links.csv", blob);

    expect(create).toHaveBeenCalledWith(blob);
    expect(clicks).toHaveLength(1);
    expect(clicks[0].download).toBe("links.csv");
    expect(clicks[0].getAttribute("href")).toBe("blob:test");
    expect(clicks[0].isConnected).toBe(false);
    expect(revoke).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1000);
    expect(revoke).toHaveBeenCalledWith("blob:test");
  });
});
