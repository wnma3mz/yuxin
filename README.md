# 余薪 Yuxin

[![CI](https://github.com/wnma3mz/yuxin/actions/workflows/ci.yml/badge.svg)](https://github.com/wnma3mz/yuxin/actions/workflows/ci.yml)

一个每秒更新的本地工作仪表盘：看看今天赚了多少、多久下班，以及下一个假期还有多远。既可在终端使用，也可通过轻量本机页面查看；退休和资产估算按需开启。

```text
╭─ 余薪 YUXIN ─────────────────────── 2026-07-16 15:00:00 ─╮
│ ● 正在上班 (三点几啦，饮茶先！)      刷新 1s  本地数据 ✓ │
├──────────────────────────────────────────────────────────┤
│                         今日入账                         │
│                         ¥227.27                          │
│                      ↗ +¥0.01 / 秒                      │
│ 下班 3h 00m  预计 ¥363.64                                │
│ █████████████████████████████████░░░░░░░░░░░░░░░ 62.5% │
│ 距端午节最后一天 25 天 · 距中秋节 71 天                  │
╰───────────────────────────────────────────────────────────╯
 [e] 配置  [r] 刷新  [s] 演示  [u] 单位  [d] 详情  [?] 帮助  [q] 退出
```

## 直接运行

直接下载对应系统的最新 ZIP，解压后运行。这些链接会自动跟随 GitHub 上的 Latest Release，无需记住版本号。

> 当前路线版仍在开发，尚未创建可下载的正式 Release；完成验证并发布后，下列安装命令和最新版直链才会生效。

macOS 或 Linux 也可以用一条命令自动识别平台并安装：

```bash
curl -fsSL https://raw.githubusercontent.com/wnma3mz/yuxin/main/scripts/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/wnma3mz/yuxin/main/scripts/install.ps1 | iex
```

| 系统 | 最新版 |
| --- | --- |
| Apple Silicon Mac | [直接下载](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-macos-arm64.zip) |
| Intel Mac | [直接下载](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-macos-x86_64.zip) |
| 64 位 Windows | [直接下载](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-windows-x86_64.zip) |
| 64 位 Linux | [直接下载](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-linux-x86_64.zip) |

Yuxin 是原生单文件程序，目标电脑不需要安装 Go、Python 或其他运行时。macOS 或 Linux 首次运行：

```bash
chmod +x yuxin
./yuxin
```

Windows 直接运行 `yuxin.exe`。如果 macOS 拦截未签名程序，请先核对 Release 中的 SHA-256，再按系统提示授权。

每个平台的 ZIP 还包含非终端启动入口：macOS 使用 `Open-Yuxin.app`，Windows 使用 `Open-Yuxin.vbs`，Linux 使用 `Open-Yuxin.desktop`（部分桌面环境首次需要标记为可信）。它们会直接打开轻量本机页面，不额外提供第二套界面。页面只监听 `127.0.0.1`，启动链接带一次性随机密钥；无密钥只能看到合成数据，显示真实数据还需要二次确认。

首次启动不会提问。程序会创建 `~/.config/yuxin/config.toml`，使用默认配置直接进入仪表盘。以后可运行 `yuxin update` 检查并安装最新正式版，下载完成后会先校验 SHA-256。如果当前年份与随包节假日数据不一致，程序会在每次启动时提醒使用 `yuxin update --force` 重新安装 Latest Release。

## 可以看到什么

- 今日收入按秒增长，同时展示工时、下班倒计时和工作进度。
- 使用随程序发布的年度节假日数据展示假期进度，刷新时不联网。
- 可选地估算渐进式延迟退休年月、退休进度和剩余工作日。
- 可选地汇总现金账户，计算实时余额和“现在退休每天可花”。
- 按 `s` 切换到固定合成数据，可安全截图展示全部功能。
- 使用 `yuxin share` 生成固定宽度纯文本卡片；默认合成数据，`--real` 才会读取真实数据。
- 金额和预计退休年月可以分别隐藏；退休模块支持完整/轻量模式及年、月、日、工作日单位。
- 配置支持导出、校验后导入和确认后清除，敏感数据始终保存在本机。
- 自动适配宽屏、窄屏和低高度终端；管道输出自动切换为单次快照。

默认值是月薪 8,000 元、每月 22 个工作日、周一至周五 09:00–18:00、午休 12:00–13:00、每秒刷新。退休和资产模块默认关闭。

## 修改配置

在仪表盘按 `e`，或者运行 `yuxin config`。配置按模块修改，不会要求重新填写全部内容：

```text
1 薪资      月薪 ¥8,000.00
2 工作时间  09:00–18:00，周 1,2,3,4,5
3 刷新      1s
4 退休      关闭
5 资产      0 个账户，合计 ¥0.00
6 隐私显示  显示全部
```

金额支持 `20w`、`20万`、`200k` 和 `200,000`。退休资料可以手动填写，也可以输入身份证自动解析出生日期和性别；程序只保存解析结果，不保存身份证原号码。

## 常用命令

```text
yuxin                 启动每秒刷新的仪表盘
yuxin once            输出一次无 ANSI 快照
yuxin config          分模块修改配置
yuxin config export FILE  导出配置并提示敏感字段
yuxin config import FILE  校验后导入配置
yuxin config clear        确认后清除本地配置
yuxin doctor          检查配置和本地数据
yuxin share           生成合成数据概览分享卡
yuxin share --card workday  生成工作日倒计时卡
yuxin share --real    显式生成真实数据分享卡
yuxin web             打开仅监听本机的轻量页面
yuxin update          安装 GitHub 最新正式版
yuxin update --force  强制重装 Latest Release 并刷新随包数据
yuxin --interval 2    本次改为每两秒刷新
yuxin --config FILE   使用指定 TOML 文件
yuxin --version       显示版本号
```

仪表盘快捷键：`e` 配置、`r` 立即刷新、`s` 隐私演示、`u` 切换退休单位、`d` 计算口径、`?` 帮助、`q` 退出。演示模式不读取工资、资产或出生日期配置，再次按 `s` 返回真实数据。

## 数据与计算口径

所有配置都保存在本机。Yuxin 不连接银行、企业系统或其他账户，也不会在刷新或打开本机页面时调用外部 API。只有用户主动运行 `yuxin update` 时才会访问 GitHub。

```text
实时余额 = 所有账户余额 + 今日已赚工资
可支配余额 = 实时余额 - 应急保留金
现在退休每天可花 = 可支配余额 ÷ 距退休日历天数
退休进度 = 18 岁至今天的天数 ÷ 18 岁至预计退休月的总天数
```

退休日期按 2025 年起实施的渐进式延迟法定退休年龄规则估算。结果不包含个税、奖金、利息、通胀、养老金和未来政策变化，不构成财务建议。

## 从源码构建

需要 Go 1.22 或更高版本，项目只使用标准库：

```bash
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w -buildid=" -o yuxin ./cmd/yuxin
./yuxin
```

终端设计说明见 [docs/terminal-ui.md](docs/terminal-ui.md)，[产品与发布路线](docs/roadmap.md) 记录了实施状态，版本变化见 [CHANGELOG.md](CHANGELOG.md)。

## 发布

推送与 `internal/app/VERSION` 一致的版本标签后，GitHub Actions 会测试、构建并创建 Latest Release：

```bash
version=$(tr -d '[:space:]' < internal/app/VERSION)
git tag "v$version"
git push origin "v$version"
```

## License

[MIT](LICENSE)

欢迎参与项目，提交前请阅读 [贡献指南](.github/CONTRIBUTING.md)。安全问题请按 [安全政策](.github/SECURITY.md) 私密报告。
