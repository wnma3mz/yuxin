# 余薪 Yuxin

[![CI](https://github.com/wnma3mz/yuxin/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/wnma3mz/yuxin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/wnma3mz/yuxin?display_name=tag&logo=github&label=release)](https://github.com/wnma3mz/yuxin/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/wnma3mz/yuxin?logo=go&logoColor=white)](https://github.com/wnma3mz/yuxin/blob/main/go.mod)
[![License](https://img.shields.io/github/license/wnma3mz/yuxin?logo=opensourceinitiative&logoColor=white)](LICENSE)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-64748b?logo=gnometerminal&logoColor=white)](https://github.com/wnma3mz/yuxin/releases/latest)

![Yuxin 终端演示](https://raw.githubusercontent.com/wnma3mz/yuxin/main/docs/assets/yuxin-demo.gif)

余薪 = 余下的薪水，亦是余生工作的动力。

摸鱼有数，下班有期。

日盼下班，终盼退休。

Yuxin 是一个本地离线运行的终端工作仪表盘，实时展示今日入账、下班倒计时和节假日进度，并按需提供轻量退休倒计时、存款显示和隐私演示。

## 直接运行

macOS 或 Linux 用一条命令自动识别平台并安装：

```bash
curl -fsSL https://raw.githubusercontent.com/wnma3mz/yuxin/main/scripts/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/wnma3mz/yuxin/main/scripts/install.ps1 | iex
```

安装脚本只从 GitHub Latest Release 下载对应平台的 ZIP 和 SHA-256，校验后安装到用户目录；不会读取或上传 Yuxin 配置。

直接下载对应系统的最新 ZIP，解压后运行。

| 系统                | 最新版                                                                                        |
| ----------------- | ------------------------------------------------------------------------------------------ |
| Apple Silicon Mac | [直接下载](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-macos-arm64.zip)    |
| Intel Mac         | [直接下载](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-macos-x86_64.zip)   |
| 64 位 Windows      | [直接下载](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-windows-x86_64.zip) |
| 64 位 Linux        | [直接下载](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-linux-x86_64.zip)   |

Yuxin 是原生单文件程序，目标电脑不需要安装 Go、Python 或其他运行时。macOS 或 Linux 首次运行：

```bash
chmod +x yuxin
./yuxin
```

Windows 直接运行 `yuxin.exe`。如果 macOS 拦截未签名程序，请先核对 Release 中的 SHA-256，再按系统提示授权。

首次启动不会提问。程序会创建 `~/.config/yuxin/config.toml`，使用默认配置直接进入仪表盘。以后可运行 `yuxin update` 检查并安装最新正式版，下载完成后会先校验 SHA-256。如果当前版本缺少本年度节假日数据，程序会在每次启动时提醒使用 `yuxin update` 检查新版本。

## 可以看到什么

- 今日收入按秒增长，同时展示工时、下班倒计时和工作进度。
- 使用随程序发布的年度节假日数据展示假期进度，刷新时不联网。
- 退休倒计时填写年龄或出生年月与性别后自动估算，分三行显示剩余总年数、总月数和总天数。
- 可选地填写存款和一个“目标每月可花”，显示实时余额、用当前存款撑到退休的日月年预算，以及达到目标所需的存款进度和还差金额；目标卡片需同时开启退休倒计时。
- 完整存款面板会在标题中按“撑到退休每天可花”显示一条仅供娱乐的生活体感彩蛋；隐私模式和紧凑布局自动隐藏。
- 按 `v` 在本地数据、固定合成演示和计算详情之间循环，演示画面可安全截图。
- 使用 `yuxin share` 生成固定宽度纯文本卡片；默认合成数据，`--real` 才会读取真实数据。
- 按 `p` 在关闭、隐藏金额和存款、再同时隐藏退休三个档位间循环；顶部口号可自定义。
- 配置支持导出、校验后导入和确认后清除，敏感数据始终保存在本机。
- 自动适配宽屏、窄屏和低高度终端；管道输出自动切换为单次快照。

默认值是月薪 8,000 元、每月 22 个工作日、周一至周五 09:00–18:00、午休 12:00–13:00、每秒刷新。退休倒计时和存款默认关闭。

## 修改配置

在仪表盘按 `e`，或者运行 `yuxin config`。配置按模块修改，不会要求重新填写全部内容：

```text
1 今日入账  月薪 ¥8,000.00
2 工作时间  09:00–18:00，周 1,2,3,4,5
3 退休倒计时 关闭
4 存款       关闭
5 更多设置   刷新 1s
```

- 金额支持 `20w`、`20万`、`200k` 和 `200,000`
- 年龄或出生日期可输入 `30`、`1995-06` 或 `1995-06-18`，再选择性别自动估算退休日期；输入 `0` 关闭
- 午休只填分钟数，输入 `0` 不扣除
- 存款输入 `0` 关闭，目标月支出输入 `0` 则关闭目标

## 常用命令

```text
yuxin                 启动每秒刷新的仪表盘
yuxin once            输出一次无 ANSI 快照
yuxin config          分模块修改配置
yuxin config export FILE  导出配置并提示敏感字段
yuxin config import FILE  校验后导入配置
yuxin config clear        确认后清除本地配置
yuxin doctor          检查配置和本地数据
yuxin doctor --strict 缺少本年度节假日数据时返回非零状态
yuxin share           生成合成数据概览分享卡
yuxin share --card workday  生成工作日倒计时卡
yuxin share --real    显式生成真实数据分享卡
yuxin update          安装 GitHub 最新正式版
yuxin update --force  强制重装 Latest Release 并刷新随包数据
yuxin uninstall       卸载程序并保留本地配置
yuxin uninstall --purge  卸载程序并清除本地配置
yuxin --interval 2    本次改为每两秒刷新
yuxin --config FILE   使用指定 TOML 文件
yuxin --version       显示版本号
```

仪表盘快捷键：`e` 配置、`p` 切换并保存隐私档位、`r` 立即刷新、`v` 循环本地数据 → 演示数据 → 计算详情、`q` 退出。底部会持续显示“隐私·金额 / 隐私·全部”和“演示 / 详情”等当前状态；窄屏自动退回短标签。演示画面的标题会持续显示“演示模式”，且不读取工资、存款或退休配置；详情画面回到本地数据并继续遵守隐私档位。

## 数据与计算口径

所有配置都保存在本机。Yuxin 不连接银行、企业系统或其他账户，刷新仪表盘时不会调用外部 API。只有用户主动运行 `yuxin update` 时才会访问 GitHub。

```text
今日入账 = 当日有效工作秒数 × 每秒收入
退休日期 = 出生日期 + 按性别估算的渐进式退休年龄
剩余总年数/总月数 = 剩余天数 ÷ 365.2425 / 30.436875，向下取整
实时存款余额 = 手动填写的存款 + 今日已入账
撑到退休每天可花 = 实时存款余额 ÷ 距退休的日历天数
撑到退休每月/每年可花 = 每天可花 × 30.436875 / 365.2425
```

退休估算参考[《国务院关于实施渐进式延迟法定退休年龄的决定》](https://www.gov.cn/yaowen/liebiao/202409/content_6974294.htm)。程序不读取身份证或参加工作年份。只填年龄时，以当天作为估算生日；女性默认按原 55 岁口径估算，旧配置中的原 50 岁口径仍兼容。结果不构成政策或财务建议。

## 从源码构建

需要 Go 1.22 或更高版本，项目只使用标准库：

```bash
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w -buildid=" -o yuxin ./cmd/yuxin
./yuxin
```

项目已在 README 中提供使用固定合成数据的终端演示 GIF；安装 `asciinema` 和 `agg` 后，可运行 `scripts/render-demo.sh` 按当前界面重新生成。另行安装 FFmpeg 后，`scripts/render-promo.sh [OUTPUT]` 可生成 1080p 宣传视频评审稿。

终端设计说明见 [docs/terminal-ui.md](https://github.com/wnma3mz/yuxin/blob/main/docs/terminal-ui.md)，[演示与宣传视频计划](https://github.com/wnma3mz/yuxin/blob/main/docs/demo-video-plan.md) 记录了素材路线，[产品与发布路线](https://github.com/wnma3mz/yuxin/blob/main/docs/roadmap.md) 记录了实施状态，版本变化见 [CHANGELOG.md](https://github.com/wnma3mz/yuxin/blob/main/CHANGELOG.md)。

## 发布

推送与 `internal/app/VERSION` 一致的版本标签后，GitHub Actions 会测试、构建并创建 Latest Release：

```bash
version=$(tr -d '[:space:]' < internal/app/VERSION)
git tag "v$version"
git push origin "v$version"
```

## License

[MIT](LICENSE)

欢迎参与项目，提交前请阅读 [贡献指南](https://github.com/wnma3mz/yuxin/blob/main/.github/CONTRIBUTING.md)。安全问题请按 [安全政策](https://github.com/wnma3mz/yuxin/blob/main/.github/SECURITY.md) 私密报告。
