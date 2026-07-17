import "./styles.css";
import holidays2026Raw from "../../internal/app/data/holidays-2026.json?raw";
import { createPublicDataClient } from "./api";
import { getDemoDashboard, getDemoMessages, shouldShowDemoData } from "./mock";
import {
  formatMoney,
  formatWorkMinutes,
  characterBar,
  contributionInterval,
  layFlatBudget,
  messageKindLabels,
  sampleLabel,
  salaryPulseAt,
  type ContributionDraft,
  type DashboardData,
  type DistributionMetric,
  type DistributionItem,
  type PublicMessage,
  type HolidayCalendarData,
} from "./model";
import { validateContribution } from "./validation";

if (window.top !== window.self) {
  const warning = document.createElement("main");
  warning.className = "embed-warning";
  warning.textContent = "为保护匿名提交确认，余薪看板不支持嵌入其他页面。";
  document.body.replaceChildren(warning);
  throw new Error("embedded rendering is disabled");
}

function element<T extends HTMLElement>(id: string): T {
  const found = document.getElementById(id);
  if (!found) throw new Error(`页面缺少 #${id}`);
  return found as T;
}

function setText(id: string, value: string): void {
  element(id).textContent = value;
}

const intervalComments: Record<DistributionMetric, Record<string, string>> = {
  salary: {
    "3千以下": "工资像来过，但没有完全来。",
    "3–5千": "精打细算，是这一区间的隐藏技能。",
    "5–8千": "生活能转起来，余额还得慢慢攒。",
    "8千–1.2万": "看起来还行，前提是别看房租。",
    "1.2–2万": "工资开始体面，时间未必从容。",
    "2–3万": "薪资往上走，消息免打扰也很重要。",
    "3万以上": "大佬也摸鱼，只是时薪更有底气。",
  },
  workHours: {
    "6小时以下": "工时很轻，愿会议也同样克制。",
    "6–8小时": "班味适中，下班还能认出自己。",
    "8–10小时": "标准工时的边界，总有一点弹性。",
    "10–12小时": "今天的晚霞，大概率又在通勤路上。",
    "12小时以上": "这不是工时，是耐力赛。请保重身体。",
  },
  savings: {
    "1万以下": "先攒应急垫，躺平计划暂缓加载。",
    "1–5万": "小有缓冲，但离任性辞职还有距离。",
    "5–10万": "拒绝一次烂需求，底气多了一点。",
    "10–30万": "存款开始兑换成选择权。",
    "30–100万": "工作的意义，逐渐从生存变成选择。",
    "100万以上": "恭喜，人生已经有了备选按钮。",
  },
  retirement: {
    "10年以内": "终点开始看得见，日历终于没那么抽象。",
    "11–20年": "倒计时已进入可以认真规划的区间。",
    "21–30年": "路还长，先把今天的班准时下了。",
    "31–40年": "退休很远，下班必须近一点。",
    "40年以上": "先别遥望退休，今天也值得按时生活。",
  },
};

const matrixDetails: Record<string, { subtitle: string; comment: string }> = {
  "钱多事少": { subtitle: "神仙区", comment: "收入和时间同时站在这边，这份班有点东西。" },
  "钱多事多": { subtitle: "高薪耐力赛", comment: "钱到位了，时间也交代了，别忘了给生活留份额度。" },
  "钱少事少": { subtitle: "松弛观察区", comment: "收入还在加载，好在下班后的时间还属于自己。" },
  "钱少事多": { subtitle: "纯牛马区", comment: "钱和时间都站在对面，建议先保住身体和退路。" },
  "摸鱼仙人": { subtitle: "薪资一般，躺平底气很足", comment: "当下赚得不算多，但过去的积累已经开始换回选择权。" },
  "隐形富豪": { subtitle: "能赚，也能躺", comment: "收入和躺平预算都高于中位数，下一步可能是决定为什么工作。" },
  "生存副本": { subtitle: "先攒一点退路", comment: "当下和未来都还在积累期，别让短期焦虑变成长期透支。" },
  "高薪长跑": { subtitle: "赚得不少，离躺还远", comment: "高薪不一定立刻换来自由，也可能是退休倒计时还很长。" },
};

function renderChartComment(metric: DistributionMetric, label: string, container: HTMLElement): void {
  container.querySelectorAll(".bar-row.is-active").forEach((row) => row.classList.remove("is-active"));
  const comment = element(`comment-${metric}`);
  comment.textContent = `区间嘴替 · ${intervalComments[metric][label] ?? "这个区间暂时保持沉默。"}`;
  comment.classList.add("is-active");
}

function renderBars(id: string, items: DistributionItem[], metric: DistributionMetric): void {
  const container = element(id);
  container.replaceChildren();
  const maximum = Math.max(...items.map((item) => item.count), 1);
  for (const item of items) {
    const row = document.createElement("button");
    row.type = "button";
    row.className = "bar-row";
    row.dataset.label = item.label;
    row.setAttribute("aria-label", `${item.label}：${item.count} 份。点击显示区间嘴替`);
    const label = document.createElement("span");
    label.className = "bar-label";
    const labelMain = document.createElement("span");
    labelMain.className = "bar-label-main";
    labelMain.textContent = item.label;
    label.append(labelMain);
    const track = document.createElement("span");
    track.className = "bar-track";
    track.setAttribute("aria-hidden", "true");
    for (const [index, glyph] of [...characterBar(item.count, maximum)].entries()) {
      const cell = document.createElement("i");
      cell.className = glyph === "█" ? "bar-glyph is-filled" : "bar-glyph";
      cell.style.setProperty("--glyph-index", index.toString());
      cell.textContent = glyph;
      track.append(cell);
    }
    const count = document.createElement("strong");
    count.textContent = item.count.toLocaleString("zh-CN");
    row.append(label, track, count);
    row.addEventListener("click", () => {
      renderChartComment(metric, item.label, container);
      row.classList.add("is-active");
    });
    container.append(row);
  }
}

function renderMatrix(id: string, items: DistributionItem[], commentID: string): void {
  const container = element(id);
  container.replaceChildren();
  const total = items.reduce((sum, item) => sum + item.count, 0);
  for (const item of items) {
    const detail = matrixDetails[item.label] ?? { subtitle: "聚合样本", comment: "这个象限暂时保持沉默。" };
    const percentage = total > 0 ? item.count / total * 100 : 0;
    const cell = document.createElement("button");
    cell.type = "button";
    cell.className = "matrix-cell";
    cell.dataset.label = item.label;
    cell.setAttribute("aria-label", `${item.label}：${percentage.toFixed(0)}%，${item.count} 份聚合样本`);
    const label = document.createElement("strong");
    label.textContent = item.label;
    const subtitle = document.createElement("small");
    subtitle.textContent = detail.subtitle;
    const value = document.createElement("b");
    value.textContent = `${percentage.toFixed(0)}%`;
    cell.append(label, subtitle, value);
    cell.addEventListener("click", () => {
      container.querySelectorAll(".matrix-cell.is-active").forEach((candidate) => candidate.classList.remove("is-active"));
      cell.classList.add("is-active");
      setText(commentID, `象限旁白 · ${detail.comment}`);
    });
    container.append(cell);
  }
}

let salaryPulseHourly: number | null = null;
let salaryPulseDailyMinutes: number | null = null;
const holidayCalendar = JSON.parse(holidays2026Raw) as HolidayCalendarData;

function clockTime(totalMinutes: number): string {
  const hours = Math.floor(totalMinutes / 60) % 24;
  const minutes = Math.floor(totalMinutes % 60);
  return `${hours.toString().padStart(2, "0")}:${minutes.toString().padStart(2, "0")}`;
}

function updateSalaryPulse(now = new Date()): void {
  const pulse = salaryPulseAt(salaryPulseHourly, salaryPulseDailyMinutes, now, holidayCalendar);
  if (pulse.phase === "missing" || pulse.earnedCny === null) {
    setText("metric-live-income", "—");
    setText("pulse-status", "等待公开样本");
    return;
  }
  setText("metric-live-income", `+¥${pulse.earnedCny.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`);
  setText("pulse-window", `统一口径 ${clockTime(pulse.startMinutes)}–${clockTime(pulse.endMinutes)} · 含工作日与调休`);
  const status: Record<typeof pulse.phase, string> = {
    rest: `${pulse.restLabel ?? "今日"}休息，工资脉冲暂停`,
    before: `等待 ${clockTime(pulse.startMinutes)} 开工`,
    working: "样本工资正在跳动",
    lunch: "午休中，工资暂停跳动",
    after: "今日样本工资已收官",
  };
  setText("pulse-status", status[pulse.phase]);
}

function resetSalaryPulse(hourlyCny: number | null, dailyWorkMinutes: number | null): void {
  salaryPulseHourly = hourlyCny;
  salaryPulseDailyMinutes = dailyWorkMinutes;
  updateSalaryPulse();
}

window.setInterval(() => updateSalaryPulse(), 1000);

let insightMode: "work" | "chill" = "work";
let insightView: "matrix" | "detail" = "matrix";
let dashboardHasData = false;
let chillMatrixMeta = "等待公开样本";
let chillDetailMeta = "等待公开样本";

function applyInsightStage(): void {
  const workActive = insightMode === "work";
  element("work-insight-panel").hidden = !dashboardHasData || !workActive;
  element("chill-insight-panel").hidden = !dashboardHasData || workActive;
  element("chart-grid").hidden = insightView !== "matrix";
  element("distribution-grid").hidden = insightView !== "detail";
  element("chill-matrix-section").hidden = insightView !== "matrix";
  element("chill-distribution-grid").hidden = insightView !== "detail";
  setText("chill-view-meta", insightView === "matrix" ? chillMatrixMeta : chillDetailMeta);
  document.querySelectorAll<HTMLButtonElement>("[data-insight-mode]").forEach((button) => {
    const selected = button.dataset.insightMode === insightMode;
    button.setAttribute("aria-selected", String(selected));
    button.tabIndex = selected ? 0 : -1;
  });
  document.querySelectorAll<HTMLButtonElement>("[data-insight-view]").forEach((button) => {
    button.setAttribute("aria-pressed", String(button.dataset.insightView === insightView));
  });
}

function renderDashboard(data: DashboardData, demo = false): void {
  setText("metric-total", data.totalSubmissions > 0 ? `${data.totalSubmissions.toLocaleString("zh-CN")}${demo ? "*" : "+"}` : "—");
  setText("metric-salary", formatMoney(data.metrics.medianSalaryCny));
  setText("metric-hours", formatWorkMinutes(data.metrics.medianDailyWorkMinutes));
  setText("metric-hourly", formatMoney(data.metrics.medianHourlyIncomeCny));
  resetSalaryPulse(data.metrics.medianHourlyIncomeCny, data.metrics.medianDailyWorkMinutes);
  setText("metric-salary-samples", demo ? "固定演示口径" : sampleLabel(data.metrics.salarySampleCount));
  setText("metric-hours-samples", demo ? "固定演示口径" : sampleLabel(data.metrics.salarySampleCount));
  renderPublicLayFlat(data.metrics.medianLayFlatDailyCny, data.metrics.layFlatSampleCount);
  setText("savings-samples", sampleLabel(data.metrics.savingsSampleCount));
  setText("retirement-samples", sampleLabel(data.metrics.retirementSampleCount));
  setText("dashboard-update", demo ? "固定演示数据 · 非真实用户样本" : data.updatedDate ? `数据更新至 ${data.updatedDate}` : "等待公开数据");
  setText("work-matrix-thresholds", `中位线：${formatMoney(data.metrics.medianSalaryCny)} × ${formatWorkMinutes(data.metrics.medianDailyWorkMinutes)}`);
  chillMatrixMeta = `中位线：${formatMoney(data.metrics.medianSalaryCny)} × 每天 ${formatMoney(data.metrics.medianLayFlatDailyCny)}`;
  chillDetailMeta = `存款 ${sampleLabel(data.metrics.savingsSampleCount)} · 退休 ${sampleLabel(data.metrics.retirementSampleCount)}`;
  renderMatrix("matrix-work", data.matrices.workValue, "comment-work-matrix");
  const chillVisualOrder: Record<string, number> = { "隐形富豪": 0, "高薪长跑": 1, "摸鱼仙人": 2, "生存副本": 3 };
  renderMatrix("matrix-chill", [...data.matrices.chillIndex].sort((left, right) => (chillVisualOrder[left.label] ?? 99) - (chillVisualOrder[right.label] ?? 99)), "comment-chill-matrix");
  renderBars("chart-salary", data.distributions.salary, "salary");
  renderBars("chart-hours", data.distributions.workHours, "workHours");
  renderBars("chart-savings", data.distributions.savings, "savings");
  renderBars("chart-retirement", data.distributions.retirement, "retirement");
  element("dashboard-empty").hidden = data.totalSubmissions > 0;
  element("value-heading").hidden = data.totalSubmissions === 0;
  dashboardHasData = data.totalSubmissions > 0;
  applyInsightStage();
}

function renderPublicLayFlat(dailyCny: number | null, sampleCount: number): void {
  if (dailyCny === null) {
    setText("lay-flat-samples", "等待至少 5 份同时填写存款和退休年数的样本");
    return;
  }
  setText("lay-flat-samples", sampleLabel(sampleCount));
}

function renderMessages(messages: PublicMessage[]): void {
  const grid = element("echo-grid");
  grid.replaceChildren();
  if (messages.length > 0) {
    const accessible = document.createElement("ul");
    accessible.className = "sr-only";
    for (const message of messages) {
      const item = document.createElement("li");
      item.textContent = `${messageKindLabels[message.kind]}：${message.text}`;
      accessible.append(item);
    }
    grid.append(accessible);

    for (let rowIndex = 0; rowIndex < 3; rowIndex++) {
      const source = messages.filter((_, index) => index % 3 === rowIndex);
      const repeated = source.length > 0 ? [...source] : [...messages];
      while (repeated.length < 5) repeated.push(...(source.length > 0 ? source : messages));
      const row = document.createElement("div");
      row.className = `echo-stream${rowIndex % 2 === 1 ? " is-reverse" : ""}`;
      row.setAttribute("aria-hidden", "true");
      const track = document.createElement("div");
      track.className = "echo-track";
      for (let copy = 0; copy < 2; copy++) {
        const group = document.createElement("div");
        group.className = "echo-group";
        for (const message of repeated) {
          const token = document.createElement("span");
          token.className = `echo-token echo-${message.kind}`;
          const tag = document.createElement("b");
          tag.textContent = messageKindLabels[message.kind];
          const text = document.createElement("span");
          text.textContent = message.text;
          token.append(tag, text);
          group.append(token);
        }
        track.append(group);
      }
      row.append(track);
      grid.append(row);
    }
  }
  element("echo-empty").hidden = messages.length > 0;
}

function shuffled<T>(items: T[]): T[] {
  const result = [...items];
  for (let index = result.length - 1; index > 0; index--) {
    const target = Math.floor(Math.random() * (index + 1));
    const current = result[index]!;
    result[index] = result[target]!;
    result[target] = current;
  }
  return result;
}

function clearComparisonHighlights(): void {
  document.querySelectorAll(".bar-row.is-you").forEach((row) => {
    row.classList.remove("is-you");
    row.querySelector(".you-marker")?.remove();
  });
}

function highlightComparisonBucket(metric: "salary" | "workHours" | "savings" | "retirement", interval: string): void {
  const chartIDs = { salary: "chart-salary", workHours: "chart-hours", savings: "chart-savings", retirement: "chart-retirement" };
  const chart = document.getElementById(chartIDs[metric]);
  if (!chart) return;
  const row = Array.from(chart.querySelectorAll<HTMLElement>(".bar-row"))
    .find((candidate) => candidate.dataset.label === interval);
  if (!row) return;
  row.classList.add("is-you");
  const marker = document.createElement("b");
  marker.className = "you-marker";
  marker.textContent = "YOU";
  row.querySelector(".bar-label-main")?.append(marker);
}

function renderComparison(input: NonNullable<ReturnType<typeof validateContribution>["value"]>, dashboard: DashboardData): void {
  setText("comparison-title", dashboardIsDemo ? "你在固定演示分布中的位置" : "你在公开样本中的位置");
  const grid = element("comparison-grid");
  grid.replaceChildren();
  clearComparisonHighlights();
  const entries: Array<[string, "salary" | "workHours" | "savings" | "retirement", number, DistributionItem[]]> = [
    ["月薪", "salary", input.monthlySalaryCny, dashboard.distributions.salary],
    ["每日纯打工时长", "workHours", input.dailyWorkMinutes, dashboard.distributions.workHours],
  ];
  if (input.savingsCny !== null) entries.push(["当前存款", "savings", input.savingsCny, dashboard.distributions.savings]);
  if (input.retirementYearsRemaining !== null) entries.push(["距离退休", "retirement", input.retirementYearsRemaining, dashboard.distributions.retirement]);
  for (const [name, metric, value, distribution] of entries) {
    const interval = contributionInterval(metric, value);
    const bucket = distribution.find((item) => item.label === interval);
    const sampleTotal = distribution.reduce((total, item) => total + item.count, 0);
    const percentage = bucket && sampleTotal > 0
      ? `${(bucket.count / sampleTotal * 100).toFixed(1)}% 的${dashboardIsDemo ? "演示样本" : "样本"}在此区间`
      : "当前区间暂无样本";
    const row = document.createElement("div");
    const label = document.createElement("span");
    label.textContent = name;
    const result = document.createElement("strong");
    result.textContent = interval;
    const detail = document.createElement("small");
    detail.textContent = percentage;
    row.append(label, result, detail);
    grid.append(row);
    highlightComparisonBucket(metric, interval);
  }
  if (input.savingsCny !== null && input.retirementYearsRemaining !== null) {
    const budget = layFlatBudget(input.savingsCny, input.retirementYearsRemaining);
    const row = document.createElement("div");
    row.className = "comparison-lay-flat";
    const label = document.createElement("span");
    label.textContent = "如果现在躺平";
    const result = document.createElement("strong");
    result.textContent = budget ? `每月可花 ${formatMoney(budget.monthly)}` : "已到预计退休时间";
    const detail = document.createElement("small");
    detail.textContent = budget
      ? `每天 ${formatMoney(budget.daily)} · 每年 ${formatMoney(budget.annual)} · 仅在本地试算`
      : "剩余年数为 0，试算不再适用";
    row.append(label, result, detail);
    grid.append(row);
  }
  element("comparison-result").hidden = false;
  element("dashboard").scrollIntoView({ behavior: "smooth", block: "start" });
}

function readDraft(form: HTMLFormElement): ContributionDraft {
  const data = new FormData(form);
  const text = (name: string): string => String(data.get(name) ?? "");
  return {
    monthlySalaryCny: text("monthlySalaryCny"),
    dailyWorkHours: text("dailyWorkHours"),
    workdaysPerWeek: text("workdaysPerWeek"),
    savingsCny: text("savingsCny"),
    retirementYearsRemaining: text("retirementYearsRemaining"),
    messageKind: text("messageKind"),
    messageText: text("messageText"),
    consent: data.get("consent") === "on",
  };
}

const client = createPublicDataClient();
let currentDashboard: DashboardData | null = null;
let currentMessages: PublicMessage[] = [];
let dashboardIsDemo = false;
const notice = element("connection-notice");
const submitButton = element<HTMLButtonElement>("submit-button");

document.querySelectorAll<HTMLButtonElement>("[data-insight-mode]").forEach((button) => {
  button.addEventListener("click", () => {
    insightMode = button.dataset.insightMode === "chill" ? "chill" : "work";
    applyInsightStage();
  });
  button.addEventListener("keydown", (event) => {
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    event.preventDefault();
    insightMode = insightMode === "work" ? "chill" : "work";
    applyInsightStage();
    document.querySelector<HTMLButtonElement>(`[data-insight-mode="${insightMode}"]`)?.focus();
  });
});

document.querySelectorAll<HTMLButtonElement>("[data-insight-view]").forEach((button) => {
  button.addEventListener("click", () => {
    insightView = button.dataset.insightView === "detail" ? "detail" : "matrix";
    applyInsightStage();
  });
});

async function refreshPublicData(): Promise<void> {
  if (client.mode === "mock") {
    notice.hidden = false;
    notice.classList.add("mock-notice");
    notice.textContent = "本地 Mock 预览 · 当前展示的是固定测试数据，不会上传或公开。";
  }
  if (!client.configured) {
    notice.hidden = false;
    notice.textContent = "开发预览：尚未配置 Supabase，公开看板与匿名提交暂不可用。";
    submitButton.disabled = true;
    return;
  }
  try {
    const [liveDashboard, liveMessages] = await Promise.all([client.loadDashboard(), client.loadMessages(24)]);
    dashboardIsDemo = shouldShowDemoData(client.mode, liveDashboard.totalSubmissions);
    const coldStart = client.mode === "supabase" && dashboardIsDemo;
    const dashboard = dashboardIsDemo ? getDemoDashboard() : liveDashboard;
    const messages = dashboardIsDemo ? getDemoMessages(24) : liveMessages;
    currentDashboard = dashboard;
    currentMessages = messages;
    if (coldStart) {
      notice.hidden = false;
      notice.classList.add("cold-start-notice");
      notice.textContent = "冷启动演示 · 真实公开样本尚未满 10 份，以下为固定演示数据，不代表真实用户；匿名提交仍进入真实数据库。";
    }
    renderDashboard(dashboard, dashboardIsDemo);
    renderMessages(shuffled(messages).slice(0, 9));
    echoRefresh.disabled = messages.length === 0;
  } catch (error) {
    notice.hidden = false;
    notice.textContent = error instanceof Error ? error.message : "公开数据暂时无法读取";
  }
}

const messageInput = document.querySelector<HTMLInputElement>('input[name="messageText"]');
messageInput?.addEventListener("input", () => setText("message-count", [...messageInput.value].length.toString()));

let terminalStatusTimer = 0;
document.querySelectorAll<HTMLButtonElement>(".terminal-command").forEach((button) => {
  button.addEventListener("click", async () => {
    const command = button.dataset.command ?? "";
    const status = element("terminal-status");
    window.clearTimeout(terminalStatusTimer);
    try {
      await navigator.clipboard.writeText(command);
      status.textContent = "✓ 命令已复制到剪贴板";
    } catch {
      status.textContent = "复制失败，请手动选择命令";
    }
    terminalStatusTimer = window.setTimeout(() => {
      status.textContent = "✓ 本地区间计算完成 · 尚未上传";
    }, 2000);
  });
});

let cliCopyTimer = 0;
document.querySelectorAll<HTMLButtonElement>(".cli-command").forEach((button) => {
  button.addEventListener("click", async () => {
    const feedback = button.querySelector("small");
    const command = button.dataset.command ?? "";
    window.clearTimeout(cliCopyTimer);
    try {
      await navigator.clipboard.writeText(command);
      if (feedback) feedback.textContent = "已复制 ✓";
    } catch {
      if (feedback) feedback.textContent = "复制失败";
    }
    cliCopyTimer = window.setTimeout(() => {
      if (feedback) feedback.textContent = "复制";
    }, 2000);
  });
});

const echoRefresh = element<HTMLButtonElement>("echo-refresh");
let echoRefreshTimer = 0;
function refreshEchoes(): void {
  if (currentMessages.length === 0) return;
  const grid = element("echo-grid");
  window.clearTimeout(echoRefreshTimer);
  grid.classList.remove("is-arriving");
  grid.classList.add("is-refreshing");
  echoRefreshTimer = window.setTimeout(() => {
    renderMessages(shuffled(currentMessages).slice(0, 9));
    grid.classList.remove("is-refreshing");
    grid.classList.add("is-arriving");
    echoRefreshTimer = window.setTimeout(() => grid.classList.remove("is-arriving"), 620);
  }, 160);
}

echoRefresh.addEventListener("click", refreshEchoes);
document.addEventListener("keydown", (event) => {
  const target = event.target as HTMLElement | null;
  const isTyping = target?.matches("input, select, textarea, [contenteditable='true']") ?? false;
  if (event.key.toLowerCase() === "r" && !event.metaKey && !event.ctrlKey && !event.altKey && !isTyping) {
    event.preventDefault();
    refreshEchoes();
  }
});

const form = element<HTMLFormElement>("contribution-form");
const compareButton = element<HTMLButtonElement>("compare-button");
const confirmDialog = element<HTMLDialogElement>("submit-confirm-dialog");
let pendingContribution: NonNullable<ReturnType<typeof validateContribution>["value"]> | null = null;
compareButton.addEventListener("click", () => {
  const errorBox = element("form-error");
  errorBox.hidden = true;
  const result = validateContribution(readDraft(form), false);
  if (!result.value) {
    errorBox.textContent = result.errors.join("；");
    errorBox.hidden = false;
    return;
  }
  if (!currentDashboard) {
    errorBox.textContent = "公开聚合数据尚未加载，暂时无法比较";
    errorBox.hidden = false;
    return;
  }
  renderComparison(result.value, currentDashboard);
});

form.addEventListener("input", () => {
  element("comparison-result").hidden = true;
  clearComparisonHighlights();
});

async function submitContribution(value: NonNullable<ReturnType<typeof validateContribution>["value"]>): Promise<void> {
  const errorBox = element("form-error");
  const successBox = element("form-success");
  submitButton.disabled = true;
  submitButton.textContent = "正在匿名提交…";
  try {
    const submission = await client.submit(value);
    form.reset();
    setText("message-count", "0");
    element("comparison-result").hidden = true;
    clearComparisonHighlights();
    successBox.textContent = submission.messageAccepted === false
      ? "数值提交成功，但匿名回声未能送达；请勿重复提交数值。"
      : submission.messageAccepted === true
        ? "提交成功。数值将在隐私门槛满足后进入后续统计，匿名回声将在审核通过后展示。"
        : "提交成功。你的数值将在隐私门槛满足后进入后续公开统计。";
    successBox.hidden = false;
    await refreshPublicData();
  } catch (error) {
    errorBox.textContent = error instanceof Error ? error.message : "提交失败，请稍后重试";
    errorBox.hidden = false;
  } finally {
    submitButton.disabled = !client.configured;
    submitButton.textContent = "匿名提交";
  }
}

form.addEventListener("submit", (event) => {
  event.preventDefault();
  const errorBox = element("form-error");
  const successBox = element("form-success");
  errorBox.hidden = true;
  successBox.hidden = true;
  const result = validateContribution(readDraft(form));
  if (!result.value) {
    errorBox.textContent = result.errors.join("；");
    errorBox.hidden = false;
    return;
  }
  pendingContribution = result.value;
  if (typeof confirmDialog.showModal === "function") {
    confirmDialog.returnValue = "cancel";
    confirmDialog.showModal();
    return;
  }
  if (window.confirm("确认匿名提交？提交后无法自行找回或修改。")) {
    void submitContribution(pendingContribution);
  }
  pendingContribution = null;
});

confirmDialog.addEventListener("close", () => {
  const value = pendingContribution;
  pendingContribution = null;
  if (confirmDialog.returnValue === "confirm" && value) {
    void submitContribution(value);
  }
});

void refreshPublicData();
