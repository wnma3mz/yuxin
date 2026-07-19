import { afterEach, describe, expect, it, vi } from "vitest";
import { deriveEditCredentialVerifier, parseDashboardData, parsePublicMessages, submitPublicContribution } from "../src/api";

function dashboard() {
  return {
    totalSubmissions: 10,
    updatedDate: "2026-07-16",
    metrics: {
      medianSalaryCny: 8000,
      medianDailyWorkMinutes: 480,
      medianHourlyIncomeCny: 48,
      medianLayFlatDailyCny: 12,
      salarySampleCount: 10,
      savingsSampleCount: 5,
      retirementSampleCount: 5,
      layFlatSampleCount: 5,
    },
    distributions: {
      salary: [{ label: "5–8千", count: 5 }],
      workHours: [{ label: "8–10小时", count: 10 }],
      savings: [],
      retirement: [],
    },
    matrices: {
      workValue: [
        { label: "钱多事少", count: 5 },
        { label: "钱多事多", count: 0 },
        { label: "钱少事少", count: 5 },
        { label: "钱少事多", count: 0 },
      ],
      chillIndex: [
        { label: "摸鱼仙人", count: 0 },
        { label: "隐形富豪", count: 5 },
        { label: "生存副本", count: 0 },
        { label: "高薪长跑", count: 0 },
      ],
    },
  };
}

describe("public API response validation", () => {
  afterEach(() => vi.unstubAllGlobals());
  it("accepts the documented aggregate shape", () => {
    expect(parseDashboardData(dashboard()).totalSubmissions).toBe(10);
    expect(parsePublicMessages([{ kind: "wish", text: "希望准点下班" }])).toHaveLength(1);
    expect(parsePublicMessages([{ kind: "encourage", text: "今天也辛苦了" }])).toHaveLength(1);
  });

  it("rejects malformed or oversized public responses", () => {
    expect(() => parseDashboardData({ ...dashboard(), totalSubmissions: -1 })).toThrow("响应格式无效");
    expect(() => parseDashboardData({ ...dashboard(), matrices: { ...dashboard().matrices, workValue: [] } })).toThrow("响应格式无效");
    expect(() => parsePublicMessages([{ kind: "unknown", text: "内容" }])).toThrow("响应格式无效");
    expect(() => parsePublicMessages([{ kind: "wish", text: "愿".repeat(81) }])).toThrow("响应格式无效");
  });

  it("submits numbers and free text through separate RPC requests", async () => {
    const requests: Array<{ url: string; body: Record<string, unknown> }> = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string, options: RequestInit) => {
      requests.push({ url, body: JSON.parse(String(options.body)) as Record<string, unknown> });
      if (url.endsWith("/submit_public_data")) {
        return new Response("false", { status: 200, headers: { "content-type": "application/json" } });
      }
      return new Response(null, { status: 204 });
    }));

    const result = await submitPublicContribution({ url: "https://project.supabase.co", key: "public" }, {
      monthlySalaryCny: 8100,
      dailyWorkMinutes: 480,
      workdaysPerWeek: 5,
      savingsCny: 12000,
      retirementYearsRemaining: 30,
      messageKind: "wish",
      messageText: "希望准点下班",
    }, "A".repeat(43));

    expect(result.updated).toBe(false);
    expect(result.messageAccepted).toBe(true);
    expect(requests.map((request) => request.url)).toEqual([
      "https://project.supabase.co/rest/v1/rpc/submit_public_data",
      "https://project.supabase.co/rest/v1/rpc/submit_public_message",
    ]);
    expect(requests[0]?.body).not.toHaveProperty("p_message_text");
    expect(requests[0]?.body.p_edit_credential_hash).toMatch(/^[0-9a-f]{64}$/u);
    expect(requests[0]?.body.p_edit_credential_hash).not.toBe("A".repeat(43));
    expect(requests[1]?.body).toEqual({ p_message_kind: "wish", p_message_text: "希望准点下班" });
  });

  it("does not ask users to repeat numeric data when only the message fails", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: "moderation unavailable" }), {
        status: 503,
        headers: { "content-type": "application/json" },
      }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await submitPublicContribution({ url: "https://project.supabase.co", key: "public" }, {
      monthlySalaryCny: 8100,
      dailyWorkMinutes: 480,
      workdaysPerWeek: 5,
      savingsCny: null,
      retirementYearsRemaining: null,
      messageKind: "rant",
      messageText: "今天又开了没有结论的会",
    });

    expect(result).toEqual({ updated: false, messageAccepted: false });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("does not submit a message when the numeric request fails", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ message: "rejected" }), {
      status: 400,
      headers: { "content-type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(submitPublicContribution({ url: "https://project.supabase.co", key: "public" }, {
      monthlySalaryCny: 8100,
      dailyWorkMinutes: 480,
      workdaysPerWeek: 5,
      savingsCny: null,
      retirementYearsRemaining: null,
      messageKind: "wish",
      messageText: "希望准点下班",
    })).rejects.toThrow("rejected");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("reports when the browser credential updated an existing sample", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("true", {
      status: 200,
      headers: { "content-type": "application/json" },
    })));

    const result = await submitPublicContribution({ url: "https://project.supabase.co", key: "public" }, {
      monthlySalaryCny: 12600,
      dailyWorkMinutes: 420,
      workdaysPerWeek: 4,
      savingsCny: null,
      retirementYearsRemaining: 24,
      messageKind: null,
      messageText: null,
    }, "A".repeat(43));

    expect(result).toEqual({ updated: true, messageAccepted: null });
  });

  it("derives a stable verifier without exposing the browser credential", async () => {
    const credential = "A".repeat(43);
    const first = await deriveEditCredentialVerifier(credential);
    expect(first).toMatch(/^[0-9a-f]{64}$/u);
    expect(first).toBe(await deriveEditCredentialVerifier(credential));
    expect(first).not.toContain(credential);
    expect(await deriveEditCredentialVerifier(null)).toBeNull();
  });
});
