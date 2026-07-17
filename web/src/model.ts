export type MessageKind = "advice" | "rant" | "wish" | "encourage";

export interface ContributionInput {
  monthlySalaryCny: number;
  dailyWorkMinutes: number;
  workdaysPerWeek: number;
  savingsCny: number | null;
  retirementYearsRemaining: number | null;
  messageKind: MessageKind | null;
  messageText: string | null;
}

export interface ContributionDraft {
  monthlySalaryCny: string;
  dailyWorkHours: string;
  workdaysPerWeek: string;
  savingsCny: string;
  retirementYearsRemaining: string;
  messageKind: string;
  messageText: string;
  consent: boolean;
}

export interface DistributionItem {
  label: string;
  count: number;
}

export interface DashboardMetrics {
  medianSalaryCny: number | null;
  medianDailyWorkMinutes: number | null;
  medianHourlyIncomeCny: number | null;
  medianLayFlatDailyCny: number | null;
  salarySampleCount: number;
  savingsSampleCount: number;
  retirementSampleCount: number;
  layFlatSampleCount: number;
}

export interface DashboardData {
  totalSubmissions: number;
  updatedDate: string | null;
  metrics: DashboardMetrics;
  distributions: {
    salary: DistributionItem[];
    workHours: DistributionItem[];
    savings: DistributionItem[];
    retirement: DistributionItem[];
  };
  matrices: {
    workValue: DistributionItem[];
    chillIndex: DistributionItem[];
  };
}

export interface PublicMessage {
  kind: MessageKind;
  text: string;
}

export interface HolidayCalendarData {
  year: number;
  periods: Array<{ name: string; start: string; end: string }>;
  workdays: string[];
}

export type SalaryPulsePhase = "missing" | "rest" | "before" | "working" | "lunch" | "after";

export interface SalaryPulse {
  earnedCny: number | null;
  phase: SalaryPulsePhase;
  startMinutes: number;
  endMinutes: number;
  restLabel: string | null;
}

export type DistributionMetric = "salary" | "workHours" | "savings" | "retirement";

export const messageKindLabels: Record<MessageKind, string> = {
  advice: "建议",
  rant: "吐槽",
  wish: "许愿",
  encourage: "打气",
};

export function formatMoney(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return "—";
  return new Intl.NumberFormat("zh-CN", {
    style: "currency",
    currency: "CNY",
    maximumFractionDigits: 0,
  }).format(Math.round(value));
}

export function formatWorkMinutes(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return "—";
  const hours = value / 60;
  return `${Number.isInteger(hours) ? hours.toFixed(0) : hours.toFixed(1)} 小时`;
}

export function sampleLabel(count: number): string {
  return count > 0 ? `${count.toLocaleString("zh-CN")} 份有效样本` : "暂无样本";
}

function localDateKey(date: Date): string {
  return `${date.getFullYear()}-${(date.getMonth() + 1).toString().padStart(2, "0")}-${date.getDate().toString().padStart(2, "0")}`;
}

export function salaryPulseAt(hourlyCny: number | null, dailyWorkMinutes: number | null, now: Date, calendar: HolidayCalendarData | null): SalaryPulse {
  const startMinutes = 9 * 60;
  const lunchStartMinutes = 12 * 60;
  const lunchEndMinutes = 13 * 60;
  const morningCapacity = lunchStartMinutes - startMinutes;
  const safeDailyMinutes = dailyWorkMinutes ?? 0;
  const endMinutes = safeDailyMinutes <= morningCapacity
    ? startMinutes + safeDailyMinutes
    : lunchEndMinutes + safeDailyMinutes - morningCapacity;
  if (hourlyCny === null || dailyWorkMinutes === null) {
    return { earnedCny: null, phase: "missing", startMinutes, endMinutes, restLabel: null };
  }

  const dateKey = localDateKey(now);
  const calendarMatches = calendar?.year === now.getFullYear();
  const makeupWorkday = calendarMatches && calendar.workdays.includes(dateKey);
  const holiday = calendarMatches ? calendar.periods.find((period) => dateKey >= period.start && dateKey <= period.end) : undefined;
  const weekend = now.getDay() === 0 || now.getDay() === 6;
  if (!makeupWorkday && (holiday || weekend)) {
    return { earnedCny: 0, phase: "rest", startMinutes, endMinutes, restLabel: holiday?.name ?? "周末" };
  }

  const currentMinutes = now.getHours() * 60 + now.getMinutes() + now.getSeconds() / 60;
  let earnedMinutes = 0;
  if (currentMinutes > startMinutes) earnedMinutes = Math.min(dailyWorkMinutes, currentMinutes - startMinutes);
  if (currentMinutes > lunchStartMinutes && dailyWorkMinutes > morningCapacity) earnedMinutes = morningCapacity;
  if (currentMinutes > lunchEndMinutes && dailyWorkMinutes > morningCapacity) {
    earnedMinutes = Math.min(dailyWorkMinutes, morningCapacity + currentMinutes - lunchEndMinutes);
  }
  const phase: SalaryPulsePhase = currentMinutes < startMinutes
    ? "before"
    : currentMinutes >= endMinutes
      ? "after"
      : currentMinutes >= lunchStartMinutes && currentMinutes < lunchEndMinutes && dailyWorkMinutes > morningCapacity
        ? "lunch"
        : "working";
  return { earnedCny: hourlyCny * earnedMinutes / 60, phase, startMinutes, endMinutes, restLabel: null };
}

export interface LayFlatBudget {
  daily: number;
  monthly: number;
  annual: number;
}

export function layFlatBudget(savingsCny: number, retirementYearsRemaining: number): LayFlatBudget | null {
  if (!Number.isFinite(savingsCny) || savingsCny < 0 || !Number.isFinite(retirementYearsRemaining) || retirementYearsRemaining <= 0) return null;
  return {
    daily: savingsCny / (retirementYearsRemaining * 365.2425),
    monthly: savingsCny / (retirementYearsRemaining * 12),
    annual: savingsCny / retirementYearsRemaining,
  };
}

export function spendingMood(dailyCny: number): string {
  if (dailyCny < 1) return "馒头自由还差一点";
  if (dailyCny < 3) return "加蛋勉强，馒头管够";
  if (dailyCny < 6) return "够一瓶快乐水";
  if (dailyCny < 12) return "早餐能吃一套煎饼果子";
  if (dailyCny < 25) return "勉强点个沙县外卖";
  if (dailyCny < 50) return "工作日午餐自由";
  if (dailyCny < 100) return "疯狂星期四肆意疯狂";
  if (dailyCny < 200) return "脱离温饱，略有小康";
  return "恭喜，退休生活开始体面";
}

export function characterBar(value: number, maximum: number, cells = 18): string {
  const safeCells = Math.max(1, Math.floor(cells));
  const filled = value <= 0 || maximum <= 0
    ? 0
    : Math.min(safeCells, Math.max(1, Math.round(value / maximum * safeCells)));
  return "█".repeat(filled) + "░".repeat(safeCells - filled);
}

export function contributionInterval(metric: DistributionMetric, value: number): string {
  const boundaries: Record<typeof metric, Array<[number, string]>> = {
    salary: [[3000, "3千以下"], [5000, "3–5千"], [8000, "5–8千"], [12000, "8千–1.2万"], [20000, "1.2–2万"], [30000, "2–3万"], [Infinity, "3万以上"]],
    workHours: [[360, "6小时以下"], [480, "6–8小时"], [600, "8–10小时"], [720, "10–12小时"], [Infinity, "12小时以上"]],
    savings: [[10000, "1万以下"], [50000, "1–5万"], [100000, "5–10万"], [300000, "10–30万"], [1000000, "30–100万"], [Infinity, "100万以上"]],
    retirement: [[11, "10年以内"], [21, "11–20年"], [31, "21–30年"], [41, "31–40年"], [Infinity, "40年以上"]],
  };
  return boundaries[metric].find(([upper]) => value < upper)?.[1] ?? "—";
}
