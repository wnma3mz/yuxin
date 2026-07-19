import type { ContributionInput, DashboardData, DistributionItem, PublicMessage } from "./model";
import { createMockDataClient } from "./mock";

const defaultSupabaseURL = "https://nubeymzysjmlwgzjpstl.supabase.co";

export interface PublicDataClient {
  configured: boolean;
  mode: "mock" | "supabase" | "unconfigured";
  loadDashboard(): Promise<DashboardData>;
  loadMessages(limit?: number): Promise<PublicMessage[]>;
  submit(input: ContributionInput, editCredential: string | null): Promise<SubmissionResult>;
}

export interface SubmissionResult {
  updated: boolean;
  messageAccepted: boolean | null;
}

export interface SupabaseConfig {
  url: string;
  key: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isCount(value: unknown): value is number {
  return Number.isInteger(value) && Number(value) >= 0;
}

function isMetric(value: unknown): value is number | null {
  return value === null || (typeof value === "number" && Number.isFinite(value) && value >= 0);
}

function isDistribution(value: unknown): value is DistributionItem[] {
  return Array.isArray(value) && value.length <= 32 && value.every((item) =>
    isRecord(item)
    && typeof item.label === "string"
    && item.label.length > 0
    && item.label.length <= 32
    && isCount(item.count));
}

function isMatrix(value: unknown, labels: string[]): value is DistributionItem[] {
  return isDistribution(value)
    && value.length === labels.length
    && value.every((item, index) => item.label === labels[index]);
}

export function parseDashboardData(value: unknown): DashboardData {
  if (!isRecord(value) || !isCount(value.totalSubmissions)) throw new Error("公开看板响应格式无效");
  if (value.updatedDate !== null && (typeof value.updatedDate !== "string" || !/^\d{4}-\d{2}-\d{2}$/u.test(value.updatedDate))) {
    throw new Error("公开看板更新时间无效");
  }
  const metrics = value.metrics;
  const distributions = value.distributions;
  const matrices = value.matrices;
  if (!isRecord(metrics)
    || !isMetric(metrics.medianSalaryCny)
    || !isMetric(metrics.medianDailyWorkMinutes)
    || !isMetric(metrics.medianHourlyIncomeCny)
    || !isMetric(metrics.medianLayFlatDailyCny)
    || !isCount(metrics.salarySampleCount)
    || !isCount(metrics.savingsSampleCount)
    || !isCount(metrics.retirementSampleCount)
    || !isCount(metrics.layFlatSampleCount)
    || !isRecord(distributions)
    || !isDistribution(distributions.salary)
    || !isDistribution(distributions.workHours)
    || !isDistribution(distributions.savings)
    || !isDistribution(distributions.retirement)
    || !isRecord(matrices)
    || !isMatrix(matrices.workValue, ["钱多事少", "钱多事多", "钱少事少", "钱少事多"])
    || !isMatrix(matrices.chillIndex, ["摸鱼仙人", "隐形富豪", "生存副本", "高薪长跑"])) {
    throw new Error("公开看板响应格式无效");
  }
  return value as unknown as DashboardData;
}

export function parsePublicMessages(value: unknown): PublicMessage[] {
  if (!Array.isArray(value) || value.length > 24 || !value.every((message) =>
    isRecord(message)
    && (message.kind === "advice" || message.kind === "rant" || message.kind === "wish" || message.kind === "encourage")
    && typeof message.text === "string"
    && [...message.text].length >= 1
    && [...message.text].length <= 80)) {
    throw new Error("匿名回声响应格式无效");
  }
  return value as PublicMessage[];
}

function createSupabaseConfig(): SupabaseConfig | null {
  const url = import.meta.env.VITE_SUPABASE_URL?.trim() || defaultSupabaseURL;
  const key = import.meta.env.VITE_SUPABASE_PUBLISHABLE_KEY?.trim();
  if (!key || key.includes("your_key")) return null;
  return { url: url.replace(/\/$/, ""), key };
}

export async function callRPC<T>(config: SupabaseConfig, name: string, parameters: Record<string, unknown> = {}): Promise<T> {
  const response = await fetch(`${config.url}/rest/v1/rpc/${name}`, {
    method: "POST",
    headers: {
      apikey: config.key,
      authorization: `Bearer ${config.key}`,
      "content-type": "application/json",
    },
    body: JSON.stringify(parameters),
    redirect: "error",
  });
  if (!response.ok) {
    const detail = await response.json().catch(() => null) as { message?: string } | null;
    throw new Error(detail?.message || `HTTP ${response.status}`);
  }
  if (response.status === 204) return undefined as T;
  return await response.json() as T;
}

export async function deriveEditCredentialVerifier(editCredential: string | null): Promise<string | null> {
  if (editCredential === null) return null;
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(editCredential));
  return [...new Uint8Array(digest)].map((value) => value.toString(16).padStart(2, "0")).join("");
}

export async function submitPublicContribution(
  config: SupabaseConfig,
  input: ContributionInput,
  editCredential: string | null = null,
): Promise<SubmissionResult> {
  const editCredentialHash = await deriveEditCredentialVerifier(editCredential);
  const updated = Boolean(await callRPC<boolean>(config, "submit_public_data", {
    p_monthly_salary_cny: input.monthlySalaryCny,
    p_daily_work_minutes: input.dailyWorkMinutes,
    p_workdays_per_week: input.workdaysPerWeek,
    p_savings_cny: input.savingsCny,
    p_retirement_years_remaining: input.retirementYearsRemaining,
    p_edit_credential_hash: editCredentialHash,
  }));
  if (input.messageKind === null || input.messageText === null) return { updated, messageAccepted: null };
  try {
    await callRPC<void>(config, "submit_public_message", {
      p_message_kind: input.messageKind,
      p_message_text: input.messageText,
    });
    return { updated, messageAccepted: true };
  } catch {
    return { updated, messageAccepted: false };
  }
}

export function createPublicDataClient(): PublicDataClient {
  const useMock = import.meta.env.DEV && import.meta.env.VITE_USE_MOCK_DATA !== "false";
  if (useMock) return createMockDataClient();
  const supabase = createSupabaseConfig();
  return {
    configured: supabase !== null,
    mode: supabase ? "supabase" : "unconfigured",
    async loadDashboard() {
      if (!supabase) throw new Error("尚未配置 Supabase");
      try {
        return parseDashboardData(await callRPC<unknown>(supabase, "get_public_dashboard"));
      } catch (error) {
        throw new Error(`读取公开看板失败：${error instanceof Error ? error.message : "未知错误"}`);
      }
    },
    async loadMessages(limit = 9) {
      if (!supabase) throw new Error("尚未配置 Supabase");
      let data: Array<{ kind: PublicMessage["kind"]; text: string }>;
      try {
        data = await callRPC(supabase, "get_public_messages", { p_limit: limit });
      } catch (error) {
        throw new Error(`读取匿名回声失败：${error instanceof Error ? error.message : "未知错误"}`);
      }
      return parsePublicMessages((data ?? []).map((row) => ({
        kind: row.kind,
        text: row.text,
      })));
    },
    async submit(input, editCredential) {
      if (!supabase) throw new Error("公开数据服务尚未配置，暂时无法提交");
      try {
        return await submitPublicContribution(supabase, input, editCredential);
      } catch (error) {
        throw new Error(`匿名提交失败：${error instanceof Error ? error.message : "未知错误"}`);
      }
    },
  };
}
