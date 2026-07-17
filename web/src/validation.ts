import { type ContributionDraft, type ContributionInput, type MessageKind } from "./model";

export interface ValidationResult {
  value: ContributionInput | null;
  errors: string[];
}

const messageKinds = new Set<MessageKind>(["advice", "rant", "wish", "encourage"]);
const urlPattern = /(?:https?:\/\/|www\.)/iu;
const controlPattern = /[\u0000-\u001f\u007f]/u;

function requiredNumber(raw: string, label: string, min: number, max: number, integer: boolean): [number | null, string | null] {
  const value = Number(raw);
  if (raw.trim() === "" || !Number.isFinite(value)) return [null, `请填写${label}`];
  if (value < min || value > max) return [null, `${label}需在 ${min}–${max} 之间`];
  if (integer && !Number.isInteger(value)) return [null, `${label}需填写整数`];
  return [value, null];
}

function optionalInteger(raw: string, label: string, min: number, max: number): [number | null, string | null] {
  if (raw.trim() === "") return [null, null];
  return requiredNumber(raw, label, min, max, true);
}

function roundTo(value: number, unit: number): number {
  return Math.round(value / unit) * unit;
}

export function validateContribution(draft: ContributionDraft, requireConsent = true): ValidationResult {
  const errors: string[] = [];
  const [monthlySalaryCny, salaryError] = requiredNumber(draft.monthlySalaryCny, "月薪", 100, 10_000_000, true);
  const [dailyWorkHours, hoursError] = requiredNumber(draft.dailyWorkHours, "每日净工时", 1, 16, false);
  const [workdaysPerWeek, workdaysError] = requiredNumber(draft.workdaysPerWeek, "每周工作天数", 1, 7, true);
  const [savingsCny, savingsError] = optionalInteger(draft.savingsCny, "当前存款", 0, 1_000_000_000_000);
  const [retirementYearsRemaining, retirementError] = optionalInteger(draft.retirementYearsRemaining, "距离预计退休年数", 0, 82);

  for (const error of [salaryError, hoursError, workdaysError, savingsError, retirementError]) {
    if (error) errors.push(error);
  }

  const rawKind = draft.messageKind.trim();
  const messageKind = rawKind === "" ? null : (rawKind as MessageKind);
  const messageText = draft.messageText.trim() || null;
  if (messageKind !== null && !messageKinds.has(messageKind)) errors.push("匿名回声类型无效");
  if ((messageKind === null) !== (messageText === null)) errors.push("匿名回声需同时选择类型并填写一句话");
  if (messageText !== null) {
    if ([...messageText].length > 80) errors.push("匿名回声不能超过 80 个字符");
    if (urlPattern.test(messageText)) errors.push("匿名回声暂不支持链接");
    if (controlPattern.test(messageText)) errors.push("匿名回声只能填写单行文本");
  }
  if (requireConsent && !draft.consent) errors.push("请确认匿名贡献说明");

  if (errors.length > 0 || monthlySalaryCny === null || dailyWorkHours === null || workdaysPerWeek === null) {
    return { value: null, errors };
  }
  return {
    value: {
      monthlySalaryCny: roundTo(monthlySalaryCny, 100),
      dailyWorkMinutes: roundTo(dailyWorkHours * 60, 30),
      workdaysPerWeek,
      savingsCny: savingsCny === null ? null : roundTo(savingsCny, 1000),
      retirementYearsRemaining,
      messageKind,
      messageText,
    },
    errors: [],
  };
}
