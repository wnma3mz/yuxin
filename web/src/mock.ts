import type { ContributionInput, DashboardData, PublicMessage } from "./model";

const dashboard: DashboardData = {
  totalSubmissions: 1_280,
  updatedDate: "2026-07-16",
  metrics: {
    medianSalaryCny: 11_800,
    medianDailyWorkMinutes: 540,
    medianHourlyIncomeCny: 60,
    medianLayFlatDailyCny: 24,
    salarySampleCount: 1_280,
    savingsSampleCount: 800,
    retirementSampleCount: 640,
    layFlatSampleCount: 500,
  },
  distributions: {
    salary: [
      { label: "3千以下", count: 55 },
      { label: "3–5千", count: 120 },
      { label: "5–8千", count: 280 },
      { label: "8千–1.2万", count: 325 },
      { label: "1.2–2万", count: 270 },
      { label: "2–3万", count: 135 },
      { label: "3万以上", count: 90 },
    ],
    workHours: [
      { label: "6小时以下", count: 35 },
      { label: "6–8小时", count: 310 },
      { label: "8–10小时", count: 500 },
      { label: "10–12小时", count: 295 },
      { label: "12小时以上", count: 135 },
    ],
    savings: [
      { label: "1万以下", count: 95 },
      { label: "1–5万", count: 215 },
      { label: "5–10万", count: 190 },
      { label: "10–30万", count: 175 },
      { label: "30–100万", count: 85 },
      { label: "100万以上", count: 35 },
    ],
    retirement: [
      { label: "10年以内", count: 50 },
      { label: "11–20年", count: 140 },
      { label: "21–30年", count: 225 },
      { label: "31–40年", count: 170 },
      { label: "40年以上", count: 45 },
    ],
  },
  matrices: {
    workValue: [
      { label: "钱多事少", count: 320 },
      { label: "钱多事多", count: 315 },
      { label: "钱少事少", count: 325 },
      { label: "钱少事多", count: 320 },
    ],
    chillIndex: [
      { label: "摸鱼仙人", count: 105 },
      { label: "隐形富豪", count: 145 },
      { label: "生存副本", count: 145 },
      { label: "高薪长跑", count: 105 },
    ],
  },
};

const messages: PublicMessage[] = [
  { kind: "advice", text: "工资是公司的，时间是自己的，先把边界守住。" },
  { kind: "rant", text: "会议如果能用三句话说完，就别给它订一小时。" },
  { kind: "wish", text: "希望下一份工作，周报可以只写本周没什么好汇报。" },
  { kind: "rant", text: "下班前五分钟收到的“在吗”，通常都不是什么好消息。" },
  { kind: "advice", text: "别拿偶尔的拼命，去掩盖长期没有方向。" },
  { kind: "wish", text: "愿每一次准点下班，都不需要偷偷摸摸。" },
  { kind: "advice", text: "先攒够拒绝一次烂工作的底气。" },
  { kind: "rant", text: "有些需求最大的价值，是在评审会上被取消。" },
  { kind: "wish", text: "希望存款增长的速度，早一点超过年龄增长的速度。" },
  { kind: "rant", text: "流程存在的意义，不该只是证明流程来过。" },
  { kind: "advice", text: "工位不是人生据点，按时离开也算一种自律。" },
  { kind: "wish", text: "愿每个临时需求，都能先临时消失一下。" },
  { kind: "encourage", text: "今天也辛苦了，按时下班不需要证明什么。" },
  { kind: "encourage", text: "慢一点也没关系，生活不是按周汇报结算的。" },
];

export function getDemoDashboard(): DashboardData {
  return structuredClone(dashboard);
}

export function getDemoMessages(limit = 9): PublicMessage[] {
  return structuredClone(messages.slice(0, limit));
}

export function shouldShowDemoData(mode: "mock" | "supabase" | "unconfigured", publicSampleCount: number): boolean {
  return mode === "mock" || (mode === "supabase" && publicSampleCount < 10);
}

export function createMockDataClient() {
  return {
    configured: true,
    mode: "mock" as const,
    async loadDashboard(): Promise<DashboardData> {
      return getDemoDashboard();
    },
    async loadMessages(limit = 9): Promise<PublicMessage[]> {
      return getDemoMessages(limit);
    },
    async submit(_input: ContributionInput): Promise<{ messageAccepted: boolean | null }> {
      await new Promise((resolve) => window.setTimeout(resolve, 450));
      return { messageAccepted: _input.messageText === null ? null : true };
    },
  };
}
