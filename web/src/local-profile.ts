import { type ContributionDraft, type HolidayCalendarData } from "./model";

export const LOCAL_PROFILE_STORAGE_KEY = "yuxin.local-profile.v1";
export const CONTRIBUTION_CREDENTIAL_STORAGE_KEY = "yuxin.contribution-credential.v1";

export interface LocalProfile {
  version: 1;
  monthlySalaryCny: number;
  monthlyWorkdays: number;
  workdays: number[];
  startTime: string;
  endTime: string;
  lunchMinutes: number;
  savingsCny: number | null;
  retirementYearsRemaining: number | null;
  retirementEstimate?: {
    ageYears: number;
    sex: "male" | "female";
  } | null;
  updatedAt: string;
}

export interface LocalProfileDraft {
  monthlySalaryCny: string;
  monthlyWorkdays: string;
  workdays: string[];
  startTime: string;
  endTime: string;
  lunchMinutes: string;
  savingsCny: string;
  retirementMode: string;
  retirementAgeYears: string;
  retirementSex: string;
  retirementYearsRemaining: string;
}

export interface LocalProfileValidationResult {
  value: LocalProfile | null;
  errors: string[];
}

export type LocalWorkPhase = "before" | "working" | "after" | "rest";

export interface LocalWorkSnapshot {
  phase: LocalWorkPhase;
  restLabel: string | null;
  earnedCny: number;
  expectedCny: number;
  hourlyCny: number;
  progress: number;
  secondsUntilStart: number;
  secondsUntilEnd: number;
}

export interface DemoWorkSnapshot {
  earnedCny: number;
  expectedCny: number;
  hourlyCny: number;
  progress: number;
  secondsUntilEnd: number;
}

export interface UpcomingHoliday {
  name: string;
  daysRemaining: number;
  ongoing: boolean;
}

export interface LocalContributionOptions {
  includeSavings: boolean;
  includeRetirement: boolean;
  messageKind: string;
  messageText: string;
  consent: boolean;
}

export interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

const base64URLAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
const contributionCredentialPattern = /^[A-Za-z0-9_-]{43}$/u;

function encodeBase64URL(bytes: Uint8Array): string {
  let output = "";
  let buffer = 0;
  let bits = 0;
  for (const byte of bytes) {
    buffer = (buffer << 8) | byte;
    bits += 8;
    while (bits >= 6) {
      bits -= 6;
      output += base64URLAlphabet[(buffer >>> bits) & 63];
    }
  }
  if (bits > 0) output += base64URLAlphabet[(buffer << (6 - bits)) & 63];
  return output;
}

export function loadContributionCredential(storage: StorageLike): string | null {
  const value = storage.getItem(CONTRIBUTION_CREDENTIAL_STORAGE_KEY);
  return value && contributionCredentialPattern.test(value) ? value : null;
}

export function getOrCreateContributionCredential(
  storage: StorageLike,
  cryptoSource: Pick<Crypto, "getRandomValues"> = crypto,
): string {
  const existing = loadContributionCredential(storage);
  if (existing) return existing;
  const bytes = new Uint8Array(32);
  cryptoSource.getRandomValues(bytes);
  const credential = encodeBase64URL(bytes);
  storage.setItem(CONTRIBUTION_CREDENTIAL_STORAGE_KEY, credential);
  return credential;
}

function requiredNumber(raw: string, label: string, minimum: number, maximum: number): [number | null, string | null] {
  const value = Number(raw);
  if (raw.trim() === "" || !Number.isFinite(value)) return [null, `请填写${label}`];
  if (value < minimum || value > maximum) return [null, `${label}需在 ${minimum}–${maximum} 之间`];
  return [value, null];
}

function optionalNumber(raw: string, label: string, minimum: number, maximum: number): [number | null, string | null] {
  if (raw.trim() === "") return [null, null];
  return requiredNumber(raw, label, minimum, maximum);
}

function timeMinutes(value: string): number | null {
  const match = /^(\d{2}):(\d{2})$/u.exec(value);
  if (!match) return null;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  if (hour > 23 || minute > 59) return null;
  return hour * 60 + minute;
}

export function defaultLocalProfileDraft(): LocalProfileDraft {
  return {
    monthlySalaryCny: "8000",
    monthlyWorkdays: "22",
    workdays: ["1", "2", "3", "4", "5"],
    startTime: "09:00",
    endTime: "18:00",
    lunchMinutes: "60",
    savingsCny: "",
    retirementMode: "",
    retirementAgeYears: "30",
    retirementSex: "male",
    retirementYearsRemaining: "",
  };
}

export function profileToDraft(profile: LocalProfile): LocalProfileDraft {
  return {
    monthlySalaryCny: profile.monthlySalaryCny.toString(),
    monthlyWorkdays: profile.monthlyWorkdays.toString(),
    workdays: profile.workdays.map(String),
    startTime: profile.startTime,
    endTime: profile.endTime,
    lunchMinutes: profile.lunchMinutes.toString(),
    savingsCny: profile.savingsCny?.toString() ?? "",
    retirementMode: profile.retirementEstimate ? "estimate" : profile.retirementYearsRemaining === null ? "" : "remaining",
    retirementAgeYears: profile.retirementEstimate?.ageYears.toString() ?? "30",
    retirementSex: profile.retirementEstimate?.sex ?? "male",
    retirementYearsRemaining: profile.retirementYearsRemaining?.toString() ?? "",
  };
}

export function estimateRetirementYears(ageYears: number, sex: "male" | "female", now = new Date()): number {
  const birthYear = now.getFullYear() - ageYears;
  const birthMonthIndex = birthYear * 12 + now.getMonth();
  const baseAge = sex === "male" ? 60 : 55;
  const firstYear = sex === "male" ? 1965 : 1970;
  const firstMonthIndex = firstYear * 12;
  const delayMonths = birthMonthIndex < firstMonthIndex
    ? 0
    : Math.min(Math.floor((birthMonthIndex - firstMonthIndex) / 4) + 1, 36);
  const retirementMonthIndex = birthMonthIndex + baseAge * 12 + delayMonths;
  const currentMonthIndex = now.getFullYear() * 12 + now.getMonth();
  return Math.max(0, Math.floor((retirementMonthIndex - currentMonthIndex) / 12));
}

export function validateLocalProfileDraft(draft: LocalProfileDraft, now = new Date()): LocalProfileValidationResult {
  const errors: string[] = [];
  const [monthlySalaryCny, salaryError] = requiredNumber(draft.monthlySalaryCny, "月薪", 100, 10_000_000);
  const [monthlyWorkdays, workdaysError] = requiredNumber(draft.monthlyWorkdays, "每月工作天数", 1, 31);
  const [lunchMinutes, lunchError] = requiredNumber(draft.lunchMinutes, "午休时长", 0, 240);
  const [savingsCny, savingsError] = optionalNumber(draft.savingsCny, "当前存款", 0, 1_000_000_000_000);
  let retirementYearsRemaining: number | null = null;
  let retirementEstimate: LocalProfile["retirementEstimate"] = null;
  let retirementError: string | null = null;
  if (draft.retirementMode === "estimate") {
    const [ageYears, ageError] = requiredNumber(draft.retirementAgeYears, "当前年龄", 1, 100);
    if (ageError) retirementError = ageError;
    else if (ageYears !== null && !Number.isInteger(ageYears)) retirementError = "当前年龄需填写整数";
    else if (draft.retirementSex !== "male" && draft.retirementSex !== "female") retirementError = "请选择性别";
    else if (ageYears !== null) {
      const sex = draft.retirementSex as "male" | "female";
      retirementYearsRemaining = estimateRetirementYears(ageYears, sex, now);
      retirementEstimate = { ageYears, sex };
    }
  } else if (draft.retirementMode === "remaining") {
    [retirementYearsRemaining, retirementError] = requiredNumber(draft.retirementYearsRemaining, "距离退休年数", 0, 82);
  } else if (draft.retirementMode !== "") {
    retirementError = "请选择有效的退休估算方式";
  }
  for (const error of [salaryError, workdaysError, lunchError, savingsError, retirementError]) {
    if (error) errors.push(error);
  }

  const startMinutes = timeMinutes(draft.startTime);
  const endMinutes = timeMinutes(draft.endTime);
  if (startMinutes === null || endMinutes === null) errors.push("请填写有效的上下班时间");
  if (startMinutes !== null && endMinutes !== null && endMinutes <= startMinutes) errors.push("下班时间需晚于上班时间");
  if (startMinutes !== null && endMinutes !== null && lunchMinutes !== null && endMinutes - startMinutes <= lunchMinutes) {
    errors.push("午休时长需短于整个工作时段");
  }

  const workdays = [...new Set(draft.workdays.map(Number))].filter((day) => Number.isInteger(day) && day >= 1 && day <= 7).sort((a, b) => a - b);
  if (workdays.length === 0) errors.push("请至少选择一个工作日");
  if (monthlyWorkdays !== null && !Number.isInteger(monthlyWorkdays)) errors.push("每月工作天数需填写整数");
  if (lunchMinutes !== null && !Number.isInteger(lunchMinutes)) errors.push("午休时长需填写整数");
  if (savingsCny !== null && !Number.isInteger(savingsCny)) errors.push("当前存款需填写整数");
  if (retirementYearsRemaining !== null && !Number.isInteger(retirementYearsRemaining)) errors.push("距离退休年数需填写整数");

  if (errors.length > 0 || monthlySalaryCny === null || monthlyWorkdays === null || lunchMinutes === null) {
    return { value: null, errors };
  }
  return {
    value: {
      version: 1,
      monthlySalaryCny: Math.round(monthlySalaryCny),
      monthlyWorkdays,
      workdays,
      startTime: draft.startTime,
      endTime: draft.endTime,
      lunchMinutes,
      savingsCny: savingsCny === null ? null : Math.round(savingsCny),
      retirementYearsRemaining,
      retirementEstimate,
      updatedAt: now.toISOString(),
    },
    errors: [],
  };
}

function isLocalProfile(value: unknown): value is LocalProfile {
  if (!value || typeof value !== "object") return false;
  const profile = value as Partial<LocalProfile>;
  if (profile.version !== 1 || typeof profile.updatedAt !== "string" || Number.isNaN(Date.parse(profile.updatedAt))) return false;
  if (typeof profile.monthlySalaryCny !== "number" || typeof profile.monthlyWorkdays !== "number" || typeof profile.lunchMinutes !== "number") return false;
  if (!Array.isArray(profile.workdays) || profile.workdays.some((day) => typeof day !== "number")) return false;
  if (typeof profile.startTime !== "string" || typeof profile.endTime !== "string") return false;
  if (profile.savingsCny !== null && typeof profile.savingsCny !== "number") return false;
  if (profile.retirementYearsRemaining !== null && typeof profile.retirementYearsRemaining !== "number") return false;
  const estimate = profile.retirementEstimate;
  if (estimate !== undefined && estimate !== null && (
    typeof estimate !== "object"
    || !Number.isInteger(estimate.ageYears)
    || estimate.ageYears < 1
    || estimate.ageYears > 100
    || estimate.sex !== "male" && estimate.sex !== "female"
  )) return false;
  const draft: LocalProfileDraft = {
    monthlySalaryCny: String(profile.monthlySalaryCny ?? ""),
    monthlyWorkdays: String(profile.monthlyWorkdays ?? ""),
    workdays: Array.isArray(profile.workdays) ? profile.workdays.map(String) : [],
    startTime: typeof profile.startTime === "string" ? profile.startTime : "",
    endTime: typeof profile.endTime === "string" ? profile.endTime : "",
    lunchMinutes: String(profile.lunchMinutes ?? ""),
    savingsCny: profile.savingsCny === null ? "" : String(profile.savingsCny ?? ""),
    retirementMode: estimate ? "estimate" : profile.retirementYearsRemaining === null ? "" : "remaining",
    retirementAgeYears: estimate?.ageYears.toString() ?? "30",
    retirementSex: estimate?.sex ?? "male",
    retirementYearsRemaining: profile.retirementYearsRemaining === null ? "" : String(profile.retirementYearsRemaining ?? ""),
  };
  return validateLocalProfileDraft(draft).value !== null;
}

export function loadLocalProfile(storage: StorageLike): LocalProfile | null {
  const raw = storage.getItem(LOCAL_PROFILE_STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return isLocalProfile(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

export function saveLocalProfile(storage: StorageLike, profile: LocalProfile): void {
  storage.setItem(LOCAL_PROFILE_STORAGE_KEY, JSON.stringify(profile));
}

export function clearLocalProfile(storage: StorageLike): void {
  storage.removeItem(LOCAL_PROFILE_STORAGE_KEY);
}

function localDateKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

function configuredWeekday(date: Date): number {
  return date.getDay() === 0 ? 7 : date.getDay();
}

export function localProfileDailyWorkMinutes(profile: LocalProfile): number {
  const startMinutes = timeMinutes(profile.startTime) ?? 0;
  const endMinutes = timeMinutes(profile.endTime) ?? startMinutes;
  return Math.max(1, endMinutes - startMinutes - profile.lunchMinutes);
}

export function demoWorkSnapshotAt(now: Date): DemoWorkSnapshot {
  const cycleSeconds = 8 * 60 * 60;
  const unixSeconds = Math.floor(now.getTime() / 1000);
  const elapsedSeconds = ((unixSeconds % cycleSeconds) + cycleSeconds) % cycleSeconds;
  const expectedCny = 772.41;
  const progress = elapsedSeconds / cycleSeconds;
  return {
    earnedCny: expectedCny * progress,
    expectedCny,
    hourlyCny: expectedCny / 8,
    progress,
    secondsUntilEnd: cycleSeconds - elapsedSeconds,
  };
}

export function localProfileContributionDraft(profile: LocalProfile, options: LocalContributionOptions): ContributionDraft {
  return {
    monthlySalaryCny: profile.monthlySalaryCny.toString(),
    dailyWorkHours: (localProfileDailyWorkMinutes(profile) / 60).toString(),
    workdaysPerWeek: profile.workdays.length.toString(),
    savingsCny: options.includeSavings && profile.savingsCny !== null ? profile.savingsCny.toString() : "",
    retirementYearsRemaining: options.includeRetirement && profile.retirementYearsRemaining !== null ? profile.retirementYearsRemaining.toString() : "",
    messageKind: options.messageKind,
    messageText: options.messageText,
    consent: options.consent,
  };
}

function workdayState(profile: LocalProfile, date: Date, calendar: HolidayCalendarData | null): { working: boolean; label: string | null } {
  const dateKey = localDateKey(date);
  if (calendar?.year === date.getFullYear()) {
    if (calendar.workdays.includes(dateKey)) return { working: true, label: null };
    const holiday = calendar.periods.find((period) => dateKey >= period.start && dateKey <= period.end);
    if (holiday) return { working: false, label: holiday.name };
  }
  return profile.workdays.includes(configuredWeekday(date))
    ? { working: true, label: null }
    : { working: false, label: "休息日" };
}

export function localWorkSnapshotAt(profile: LocalProfile, now: Date, calendar: HolidayCalendarData | null): LocalWorkSnapshot {
  const startMinutes = timeMinutes(profile.startTime) ?? 0;
  const endMinutes = timeMinutes(profile.endTime) ?? startMinutes + 1;
  const workSpanMinutes = endMinutes - startMinutes;
  const netMinutes = localProfileDailyWorkMinutes(profile);
  const expectedCny = profile.monthlySalaryCny / profile.monthlyWorkdays;
  const hourlyCny = expectedCny / (netMinutes / 60);
  const day = workdayState(profile, now, calendar);
  if (!day.working) {
    return { phase: "rest", restLabel: day.label, earnedCny: 0, expectedCny, hourlyCny, progress: 0, secondsUntilStart: 0, secondsUntilEnd: 0 };
  }

  const currentMinutes = now.getHours() * 60 + now.getMinutes() + now.getSeconds() / 60;
  if (currentMinutes < startMinutes) {
    return {
      phase: "before", restLabel: null, earnedCny: 0, expectedCny, hourlyCny, progress: 0,
      secondsUntilStart: Math.max(0, Math.round((startMinutes - currentMinutes) * 60)),
      secondsUntilEnd: Math.max(0, Math.round((endMinutes - currentMinutes) * 60)),
    };
  }
  if (currentMinutes >= endMinutes) {
    return { phase: "after", restLabel: null, earnedCny: expectedCny, expectedCny, hourlyCny, progress: 1, secondsUntilStart: 0, secondsUntilEnd: 0 };
  }
  const progress = Math.min(1, Math.max(0, (currentMinutes - startMinutes) / workSpanMinutes));
  return {
    phase: "working", restLabel: null, earnedCny: expectedCny * progress, expectedCny, hourlyCny, progress,
    secondsUntilStart: 0,
    secondsUntilEnd: Math.max(0, Math.round((endMinutes - currentMinutes) * 60)),
  };
}

function parseLocalDate(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/u.exec(value);
  if (!match) return null;
  return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
}

export function upcomingHolidayAt(now: Date, calendar: HolidayCalendarData | null): UpcomingHoliday | null {
  if (!calendar || calendar.year !== now.getFullYear()) return null;
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  for (const period of calendar.periods) {
    const start = parseLocalDate(period.start);
    const end = parseLocalDate(period.end);
    if (!start || !end || end < today) continue;
    const ongoing = start <= today;
    return {
      name: period.name,
      daysRemaining: ongoing ? 0 : Math.round((start.getTime() - today.getTime()) / 86_400_000),
      ongoing,
    };
  }
  return null;
}
