import { describe, expect, it } from "vitest";
import { formatDate, formatDateTime, formatNumber, formatShortUrl, pct } from "./format";

describe("format", () => {
  it("formatDate renders short month, day, year (TZ=UTC in setup)", () => {
    expect(formatDate("2026-10-24T09:37:00Z")).toBe("Oct 24, 2026");
    // Late UTC evening would be the next day east of UTC; pinned TZ keeps it stable.
    expect(formatDate("2026-10-24T23:30:00Z")).toBe("Oct 24, 2026");
  });

  it("formatDateTime renders long date, time and short tz name", () => {
    expect(formatDateTime("2026-10-24T09:37:00Z", "Asia/Shanghai")).toBe(
      "October 24, 2026 5:37 PM GMT+8",
    );
  });

  it("formatNumber groups thousands", () => {
    expect(formatNumber(1240)).toBe("1,240");
  });

  it("pct rounds to an integer percentage", () => {
    expect(pct(1240, 3260)).toBe("38%");
    expect(pct(5, 0)).toBe("0%");
  });

  it("formatShortUrl strips the protocol", () => {
    expect(formatShortUrl("https://bit.ly/abc")).toBe("bit.ly/abc");
    expect(formatShortUrl("http://bit.ly/abc")).toBe("bit.ly/abc");
  });
});
