import { describe, expect, it } from "vitest";
import {
  CONTRIBUTION_CREDENTIAL_STORAGE_KEY,
  LOCAL_PROFILE_STORAGE_KEY,
  clearLocalProfile,
  demoWorkSnapshotAt,
  defaultLocalProfileDraft,
  estimateRetirementYears,
  getOrCreateContributionCredential,
  loadContributionCredential,
  loadLocalProfile,
  localProfileDailyWorkMinutes,
  localProfileContributionDraft,
  localWorkSnapshotAt,
  saveLocalProfile,
  upcomingHolidayAt,
  validateLocalProfileDraft,
} from "../src/local-profile";
import type { HolidayCalendarData } from "../src/model";

class MemoryStorage {
  private readonly values = new Map<string, string>();
  getItem(key: string): string | null { return this.values.get(key) ?? null; }
  setItem(key: string, value: string): void { this.values.set(key, value); }
  removeItem(key: string): void { this.values.delete(key); }
}

const calendar: HolidayCalendarData = {
  year: 2026,
  periods: [{ name: "国庆节", start: "2026-10-01", end: "2026-10-07" }],
  workdays: ["2026-10-10"],
};

function profile() {
  const result = validateLocalProfileDraft(defaultLocalProfileDraft(), new Date("2026-07-19T00:00:00Z"));
  if (!result.value) throw new Error(result.errors.join("；"));
  return result.value;
}

describe("local profile", () => {
  it("validates and normalizes the default draft", () => {
    const result = validateLocalProfileDraft(defaultLocalProfileDraft(), new Date("2026-07-19T00:00:00Z"));
    expect(result.errors).toEqual([]);
    expect(result.value).toMatchObject({
      version: 1,
      monthlySalaryCny: 8000,
      monthlyWorkdays: 22,
      workdays: [1, 2, 3, 4, 5],
      lunchMinutes: 60,
    });
    expect(localProfileDailyWorkMinutes(result.value!)).toBe(480);
  });

  it("rejects an impossible schedule and empty workdays", () => {
    const result = validateLocalProfileDraft({
      ...defaultLocalProfileDraft(),
      workdays: [],
      startTime: "18:00",
      endTime: "09:00",
    });
    expect(result.value).toBeNull();
    expect(result.errors).toContain("下班时间需晚于上班时间");
    expect(result.errors).toContain("请至少选择一个工作日");
  });

  it("round-trips through versioned browser storage", () => {
    const storage = new MemoryStorage();
    const value = profile();
    saveLocalProfile(storage, value);
    expect(loadLocalProfile(storage)).toEqual(value);
    clearLocalProfile(storage);
    expect(storage.getItem(LOCAL_PROFILE_STORAGE_KEY)).toBeNull();
  });

  it("creates and reuses a browser-only anonymous correction credential", () => {
    const storage = new MemoryStorage();
    let calls = 0;
    const cryptoSource = {
      getRandomValues(array: Uint8Array): Uint8Array {
        calls++;
        array.forEach((_, index) => { array[index] = index; });
        return array;
      },
    } as Pick<Crypto, "getRandomValues">;
    const credential = getOrCreateContributionCredential(storage, cryptoSource);
    expect(credential).toMatch(/^[A-Za-z0-9_-]{43}$/u);
    expect(loadContributionCredential(storage)).toBe(credential);
    expect(getOrCreateContributionCredential(storage, cryptoSource)).toBe(credential);
    expect(calls).toBe(1);

    clearLocalProfile(storage);
    expect(storage.getItem(CONTRIBUTION_CREDENTIAL_STORAGE_KEY)).toBe(credential);
    storage.setItem(CONTRIBUTION_CREDENTIAL_STORAGE_KEY, "invalid");
    expect(loadContributionCredential(storage)).toBeNull();
  });

  it("ignores corrupt or incompatible stored data", () => {
    const storage = new MemoryStorage();
    storage.setItem(LOCAL_PROFILE_STORAGE_KEY, "not-json");
    expect(loadLocalProfile(storage)).toBeNull();
    storage.setItem(LOCAL_PROFILE_STORAGE_KEY, JSON.stringify({ version: 2 }));
    expect(loadLocalProfile(storage)).toBeNull();
    storage.setItem(LOCAL_PROFILE_STORAGE_KEY, JSON.stringify({ ...profile(), monthlySalaryCny: "8000" }));
    expect(loadLocalProfile(storage)).toBeNull();
  });

  it("calculates before-work, working and after-work states", () => {
    const value = profile();
    expect(localWorkSnapshotAt(value, new Date(2026, 6, 20, 8, 0), calendar).phase).toBe("before");
    const working = localWorkSnapshotAt(value, new Date(2026, 6, 20, 13, 30), calendar);
    expect(working.phase).toBe("working");
    expect(working.progress).toBeCloseTo(0.5);
    expect(working.earnedCny).toBeCloseTo(working.expectedCny / 2);
    expect(localWorkSnapshotAt(value, new Date(2026, 6, 20, 19, 0), calendar).phase).toBe("after");
  });

  it("honors holidays, makeup workdays and the next holiday", () => {
    const value = profile();
    expect(localWorkSnapshotAt(value, new Date(2026, 9, 1, 10, 0), calendar)).toMatchObject({ phase: "rest", restLabel: "国庆节" });
    expect(localWorkSnapshotAt(value, new Date(2026, 9, 10, 10, 0), calendar).phase).toBe("working");
    expect(upcomingHolidayAt(new Date(2026, 8, 28), calendar)).toEqual({ name: "国庆节", daysRemaining: 3, ongoing: false });
    expect(upcomingHolidayAt(new Date(2026, 9, 2), calendar)).toEqual({ name: "国庆节", daysRemaining: 0, ongoing: true });
  });

  it("keeps demo earnings moving across reloads using the current clock", () => {
    const start = demoWorkSnapshotAt(new Date(0));
    const afterOneSecond = demoWorkSnapshotAt(new Date(1000));
    const afterFourHours = demoWorkSnapshotAt(new Date(4 * 60 * 60 * 1000));
    expect(start.earnedCny).toBe(0);
    expect(afterOneSecond.earnedCny).toBeGreaterThan(start.earnedCny);
    expect(afterFourHours.progress).toBe(0.5);
    expect(afterFourHours.earnedCny).toBeCloseTo(afterFourHours.expectedCny / 2);
  });

  it("supports estimated and manually entered retirement years", () => {
    const now = new Date(2026, 6, 19);
    expect(estimateRetirementYears(30, "male", now)).toBe(33);
    expect(estimateRetirementYears(30, "female", now)).toBe(28);

    const estimated = validateLocalProfileDraft({
      ...defaultLocalProfileDraft(),
      retirementMode: "estimate",
      retirementAgeYears: "30",
      retirementSex: "female",
    }, now);
    expect(estimated.value).toMatchObject({
      retirementYearsRemaining: 28,
      retirementEstimate: { ageYears: 30, sex: "female" },
    });

    const manual = validateLocalProfileDraft({
      ...defaultLocalProfileDraft(),
      retirementMode: "remaining",
      retirementYearsRemaining: "25",
    }, now);
    expect(manual.value).toMatchObject({ retirementYearsRemaining: 25, retirementEstimate: null });
  });

  it("keeps optional anonymous fields opt-in", () => {
    const value = { ...profile(), savingsCny: 100_000, retirementYearsRemaining: 30 };
    const hidden = localProfileContributionDraft(value, { includeSavings: false, includeRetirement: false, messageKind: "", messageText: "", consent: false });
    expect(hidden).toMatchObject({ savingsCny: "", retirementYearsRemaining: "", dailyWorkHours: "8" });
    const included = localProfileContributionDraft(value, { includeSavings: true, includeRetirement: true, messageKind: "wish", messageText: "准时下班", consent: true });
    expect(included).toMatchObject({ savingsCny: "100000", retirementYearsRemaining: "30", messageKind: "wish", consent: true });
  });
});
