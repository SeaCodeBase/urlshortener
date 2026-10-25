import { describe, expect, it } from "vitest";
import { activityDetails, activityTarget, humanizeAction } from "./activity";

describe("activity helpers", () => {
  it("humanises actions", () => {
    expect(humanizeAction("link.create")).toBe("Created link");
    expect(humanizeAction("link.update")).toBe("Updated link");
    expect(humanizeAction("link.hide")).toBe("Hid link");
    expect(humanizeAction("link.unhide")).toBe("Unhid link");
    expect(humanizeAction("link.delete")).toBe("Deleted link");
    expect(humanizeAction("link.back_half")).toBe("Changed back-half");
    expect(humanizeAction("domain.verify")).toBe("Verified domain");
    expect(humanizeAction("token.create")).toBe("Created API token");
    expect(humanizeAction("webhook.deactivate")).toBe("Deactivated webhook");
    expect(humanizeAction("webhook.activate")).toBe("Activated webhook");
    expect(humanizeAction("bulk.create")).toBe("Created bulk upload");
    expect(humanizeAction("campaign.delete")).toBe("Deleted campaign");
  });

  it("picks a readable target and formats details", () => {
    const base = { id: 1, user_id: 1, group_id: 1, action: "link.create", entity_type: "link", entity_id: 9, created_at: "" };
    expect(activityTarget({ ...base, metadata: { short_code: "abc" } })).toBe("abc");
    expect(activityTarget({ ...base, metadata: {} })).toBe("link #9");
    expect(activityDetails({ short_code: "abc", tags: ["a"] })).toEqual(["short_code: abc", 'tags: ["a"]']);
  });
});
