import { describe, expect, it } from "vitest";
import { characterBar, contributionInterval, formatMoney, formatWorkMinutes, layFlatBudget, matrixPercentage, sampleLabel, spendingMood } from "../src/model";

describe("dashboard formatting", () => {
  it("formats monetary medians as whole yuan", () => {
    expect(formatMoney(8000.4)).toContain("8,000");
    expect(formatMoney(null)).toBe("—");
  });

  it("formats work minutes without unnecessary decimals", () => {
    expect(formatWorkMinutes(480)).toBe("8 小时");
    expect(formatWorkMinutes(510)).toBe("8.5 小时");
    expect(formatWorkMinutes(null)).toBe("—");
  });

  it("distinguishes empty and populated samples", () => {
    expect(sampleLabel(1286)).toBe("1,286 份有效样本");
    expect(sampleLabel(0)).toBe("暂无样本");
  });

  it("hides matrix percentages below the public threshold", () => {
    expect(matrixPercentage(0, 0)).toBeNull();
    expect(matrixPercentage(0, 5)).toBeNull();
    expect(matrixPercentage(5, 10)).toBe("50%");
  });

  it("maps values to neutral public intervals", () => {
    expect(contributionInterval("salary", 11900)).toBe("8千–1.2万");
    expect(contributionInterval("workHours", 600)).toBe("10–12小时");
    expect(contributionInterval("savings", 1_000_000)).toBe("100万以上");
    expect(contributionInterval("retirement", 31)).toBe("31–40年");
  });

  it("renders deterministic character-grid bars", () => {
    expect(characterBar(0, 100, 10)).toBe("░░░░░░░░░░");
    expect(characterBar(1, 100, 10)).toBe("█░░░░░░░░░");
    expect(characterBar(50, 100, 10)).toBe("█████░░░░░");
    expect(characterBar(100, 100, 10)).toBe("██████████");
  });

  it("calculates a local lay-flat budget without financial assumptions", () => {
    expect(layFlatBudget(120_000, 10)).toEqual({ daily: expect.any(Number), monthly: 1000, annual: 12000 });
    expect(layFlatBudget(120_000, 0)).toBeNull();
    expect(spendingMood(0.8)).toBe("馒头自由还差一点");
    expect(spendingMood(60)).toBe("疯狂星期四肆意疯狂");
    expect(spendingMood(220)).toBe("恭喜，退休生活开始体面");
  });

});
