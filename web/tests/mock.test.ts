import { describe, expect, it } from "vitest";
import { getDemoDashboard, getDemoMessages, shouldShowDemoData, shouldShowDemoMessages } from "../src/mock";

describe("cold-start demo data", () => {
  it("uses demo data only for local mock or a pre-release public batch", () => {
    expect(shouldShowDemoData("mock", 1280)).toBe(true);
    expect(shouldShowDemoData("supabase", 0)).toBe(true);
    expect(shouldShowDemoData("supabase", 10)).toBe(false);
    expect(shouldShowDemoData("unconfigured", 0)).toBe(false);
  });

  it("uses clearly labelled demo messages only when public messages are empty", () => {
    expect(shouldShowDemoMessages("mock", 3)).toBe(true);
    expect(shouldShowDemoMessages("supabase", 0)).toBe(true);
    expect(shouldShowDemoMessages("supabase", 1)).toBe(false);
    expect(shouldShowDemoMessages("unconfigured", 0)).toBe(false);
  });

  it("returns isolated fixed data that cannot mutate later renders", () => {
    const dashboard = getDemoDashboard();
    dashboard.metrics.medianSalaryCny = 1;
    expect(getDemoDashboard().metrics.medianSalaryCny).toBe(11_800);

    const messages = getDemoMessages(2);
    messages[0]!.text = "changed";
    expect(getDemoMessages(2)[0]!.text).not.toBe("changed");
  });
});
