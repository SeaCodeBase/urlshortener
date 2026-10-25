import { describe, expect, it, vi } from "vitest";

const downloadBlob = vi.fn();
vi.mock("@/lib/download", () => ({ downloadBlob: (...args: unknown[]) => downloadBlob(...args) }));

import { BULK_TEMPLATE_CSV, BULK_TEMPLATE_HEADER, downloadBulkTemplate } from "./bulkTemplate";

describe("bulkTemplate", () => {
  it("starts with the exact header line and one example row", () => {
    expect(BULK_TEMPLATE_HEADER).toBe("long_url,title,custom_back_half,tags,domain");
    const lines = BULK_TEMPLATE_CSV.split("\n");
    expect(lines[0]).toBe("long_url,title,custom_back_half,tags,domain");
    expect(lines[1]).toBe("https://example.com/landing,Spring sale,spring-sale,promo|email,");
    expect(lines).toHaveLength(3);
    expect(lines[2]).toBe("");
  });

  it("downloads the template as bulk-upload-template.csv", async () => {
    downloadBulkTemplate();
    expect(downloadBlob).toHaveBeenCalledTimes(1);
    const [name, blob] = downloadBlob.mock.calls[0] as [string, Blob];
    expect(name).toBe("bulk-upload-template.csv");
    expect(blob.type).toBe("text/csv");
    expect(await blob.text()).toBe(BULK_TEMPLATE_CSV);
  });
});
