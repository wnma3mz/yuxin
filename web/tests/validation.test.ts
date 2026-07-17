import { describe, expect, it } from "vitest";
import type { ContributionDraft } from "../src/model";
import { validateContribution } from "../src/validation";

function validDraft(overrides: Partial<ContributionDraft> = {}): ContributionDraft {
  return {
    monthlySalaryCny: "8000",
    dailyWorkHours: "8",
    workdaysPerWeek: "5",
    savingsCny: "10000",
    retirementYearsRemaining: "30",
    messageKind: "",
    messageText: "",
    consent: true,
    ...overrides,
  };
}

describe("validateContribution", () => {
  it("parses a complete contribution", () => {
    const result = validateContribution(validDraft({ messageKind: "encourage", messageText: "今天也辛苦了。" }));
    expect(result.errors).toEqual([]);
    expect(result.value).toMatchObject({
      monthlySalaryCny: 8000,
      dailyWorkMinutes: 480,
      workdaysPerWeek: 5,
      messageKind: "encourage",
    });
  });

  it("coarsens values before they enter the upload payload", () => {
    const result = validateContribution(validDraft({
      monthlySalaryCny: "8123",
      dailyWorkHours: "8.2",
      savingsCny: "12345",
    }));
    expect(result.value).toMatchObject({
      monthlySalaryCny: 8100,
      dailyWorkMinutes: 480,
      savingsCny: 12000,
    });
  });

  it("allows local comparison without accepting upload consent", () => {
    const result = validateContribution(validDraft({ consent: false }), false);
    expect(result.errors).toEqual([]);
    expect(result.value).not.toBeNull();
  });

  it("allows optional fields to stay empty", () => {
    const result = validateContribution(validDraft({
      savingsCny: "",
      retirementYearsRemaining: "",
    }));
    expect(result.errors).toEqual([]);
    expect(result.value).toMatchObject({
      savingsCny: null,
      retirementYearsRemaining: null,
    });
  });

  it("rejects missing consent and out-of-range work data", () => {
    const result = validateContribution(validDraft({ dailyWorkHours: "17", workdaysPerWeek: "0", consent: false }));
    expect(result.value).toBeNull();
    expect(result.errors.join(" ")).toContain("每日净工时");
    expect(result.errors.join(" ")).toContain("每周工作天数");
    expect(result.errors.join(" ")).toContain("确认匿名贡献说明");
  });

  it("rejects values that would round below the database minimum", () => {
    const result = validateContribution(validDraft({ monthlySalaryCny: "49" }));
    expect(result.value).toBeNull();
    expect(result.errors.join(" ")).toContain("月薪");
  });

  it("requires the message type and text together", () => {
    expect(validateContribution(validDraft({ messageKind: "rant" })).errors).toContain("匿名回声需同时选择类型并填写一句话");
    expect(validateContribution(validDraft({ messageText: "只写正文" })).errors).toContain("匿名回声需同时选择类型并填写一句话");
  });

  it("rejects links, multiline content, and oversized messages", () => {
    expect(validateContribution(validDraft({ messageKind: "advice", messageText: "访问 https://example.com" })).errors).toContain("匿名回声暂不支持链接");
    expect(validateContribution(validDraft({ messageKind: "advice", messageText: "第一行\n第二行" })).errors).toContain("匿名回声只能填写单行文本");
    expect(validateContribution(validDraft({ messageKind: "advice", messageText: "愿".repeat(81) })).errors).toContain("匿名回声不能超过 80 个字符");
  });

  it("rejects unknown message types", () => {
    const result = validateContribution(validDraft({ messageKind: "other", messageText: "未知类型" }));
    expect(result.errors).toContain("匿名回声类型无效");
  });
});
