import { describe, expect, it } from "vitest";
import { DEFAULT_LINK_FILTERS } from "./FiltersDialog";
import { hasActiveQuery, linksUrlStateToSearch, parseLinksUrlState, toApiParams } from "./linksUrlState";

const parse = (qs: string) => parseLinksUrlState(new URLSearchParams(qs));

describe("linksUrlState campaign_id", () => {
  it("round-trips a positive integer campaign_id", () => {
    const s = parse("campaign_id=12&tags=a");
    expect(s.filters.campaignId).toBe("12");
    expect(linksUrlStateToSearch(s)).toBe("tags=a&campaign_id=12");
    expect(parse(linksUrlStateToSearch(s))).toEqual(s);
    expect(toApiParams(s).campaign_id).toBe(12);
    expect(hasActiveQuery(s)).toBe(true);
  });

  it.each(["abc", "0", "-3", "1.5", "12abc", ""])("drops campaign_id=%s", (v) => {
    const s = parse(`campaign_id=${v}`);
    expect(s.filters.campaignId).toBe("");
    expect(linksUrlStateToSearch(s)).toBe("");
    expect(toApiParams(s).campaign_id).toBeUndefined();
    expect(hasActiveQuery(s)).toBe(false);
  });

  it("omits the 'all' sentinel", () => {
    const s = { ...parse(""), filters: { ...DEFAULT_LINK_FILTERS, campaignId: "all" } };
    expect(linksUrlStateToSearch(s)).toBe("");
    expect(toApiParams(s).campaign_id).toBeUndefined();
    expect(hasActiveQuery(s)).toBe(false);
  });
});
