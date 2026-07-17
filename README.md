# 余薪 Yuxin

[![CI](https://github.com/wnma3mz/yuxin/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/wnma3mz/yuxin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/wnma3mz/yuxin?display_name=tag&logo=github&label=release)](https://github.com/wnma3mz/yuxin/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/wnma3mz/yuxin?logo=go&logoColor=white)](https://github.com/wnma3mz/yuxin/blob/main/go.mod)
[![License](https://img.shields.io/github/license/wnma3mz/yuxin?logo=opensourceinitiative&logoColor=white)](LICENSE)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-64748b?logo=gnometerminal&logoColor=white)](https://github.com/wnma3mz/yuxin/releases/latest)

![Yuxin 终端演示](https://raw.githubusercontent.com/wnma3mz/yuxin/main/docs/assets/yuxin-demo.gif)

余薪 = 余下的薪水，亦是余生工作的动力。

摸鱼有数，下班有期。

Yuxin 是一个本地离线运行的终端工作仪表盘，实时展示今日入账、下班倒计时和节假日进度，并按需提供轻量退休倒计时、存款显示和隐私演示。

## 安装与直接运行

macOS Homebrew：

```bash
brew install wnma3mz/tap/yuxin
```

macOS 或 Linux 自动安装：

```bash
curl -fsSL https://raw.githubusercontent.com/wnma3mz/yuxin/main/scripts/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/wnma3mz/yuxin/main/scripts/install.ps1 | iex
```

安装脚本只从 GitHub Latest Release 下载对应平台的 ZIP 和 SHA-256，校验后安装到用户目录；不会读取或上传 Yuxin 配置。

直接下载对应系统的最新 ZIP，解压后运行。

| 系统 | ARM64 | x86_64 |
| --- | --- | --- |
| macOS | [Apple Silicon](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-macos-arm64.zip) | [Intel](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-macos-x86_64.zip) |
| Windows | [ARM64](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-windows-arm64.zip) | [x86_64](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-windows-x86_64.zip) |
| Linux | [ARM64](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-linux-arm64.zip) | [x86_64](https://github.com/wnma3mz/yuxin/releases/latest/download/yuxin-linux-x86_64.zip) |

Yuxin 是原生单文件程序，目标电脑不需要安装 Go、Python 或其他运行时。macOS 或 Linux 首次运行：

```bash
chmod +x yuxin
./yuxin
```

Windows 直接运行 `yuxin.exe`。

### macOS 浏览器下载提示

优先使用上方 Homebrew 或自动安装命令。如果从浏览器下载 ZIP，请先解压，再双击其中的 `yuxin`。当前 macOS 发布包尚未使用 Apple Developer ID 签名和公证；如果系统拦截：

1. 先确认 ZIP 的 SHA-256 与 Release 中对应的 `.sha256` 文件一致。
2. 双击 `yuxin` 并让系统出现一次拦截提示。
3. 打开“系统设置 → 隐私与安全性”，向下滚动到“安全性”。
4. 点击与 `yuxin` 对应的“仍要打开”，输入登录密码并再次确认。该按钮通常只会在尝试启动后的约一小时内出现。
5. 完成一次授权后，以后可以像普通程序一样双击启动。

这是 Apple 为未签名程序提供的单次例外流程，可参考 [Apple 官方说明](https://support.apple.com/guide/mac-help/open-a-mac-app-from-an-unknown-developer-mh40616/mac)。只应对已核对来源和校验值的文件执行此操作。

首次启动不会提问。直接解压 ZIP 运行时，程序会自动读写与可执行文件同目录的 `yuxin.toml`，作为可随整个目录移动的便携配置。通过 Homebrew 或安装脚本使用时，程序会创建 `~/.config/yuxin/config.toml`。便携配置可能包含薪资、存款和退休信息；移动或分享解压目录前请先检查，删除该目录也会一并删除其中的配置。

以后可运行 `yuxin update` 检查并安装最新正式版，下载完成后会先校验 SHA-256。如果当前版本缺少本年度节假日数据，程序会在每次启动时提醒使用 `yuxin update` 检查新版本。

## 可以看到什么

- 今日收入按秒增长，同时展示工时、下班倒计时和工作进度。
- 使用随程序发布的年度节假日数据展示假期进度，刷新时不联网。

日盼下班，终盼退休。

- 🏁 退休倒计时填写年龄或出生年月与性别后自动估算，按年、月、天三种平行口径展示剩余时间。
- 💰 可选地填写存款和一个“目标每月可花”，显示实时余额、如果现在躺平撑到退休的每天和每月可花金额，以及达到目标所需的存款进度和还差金额；目标卡片需同时开启退休倒计时。
- 完整存款面板会在标题中按“撑到退休每天可花”显示一条仅供娱乐的生活体感彩蛋；隐私模式和紧凑布局自动隐藏。
- 按 `v` 在本地数据、固定合成演示和计算详情之间循环，演示画面可安全截图。
- 使用 `yuxin share` 生成固定宽度纯文本卡片；默认合成数据，`--real` 才会读取真实数据。
- 按 `p` 在关闭、隐藏金额和存款、再同时隐藏退休三个档位间循环；顶部口号可自定义。
- 配置支持导出、校验后导入和确认后清除，敏感数据始终保存在本机。
- 自动适配宽屏、窄屏和低高度终端；管道输出自动切换为单次快照。

默认值是月薪 8,000 元、每月 22 个工作日、周一至周五 09:00–18:00、午休 12:00–13:00、每秒刷新。退休倒计时和存款默认关闭；首次开启存款时，“目标每月可花”默认为 3,000 元。

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
yuxin share --anonymous  本地预览区间并确认后匿名贡献到公开看板
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

所有配置都保存在本机。Yuxin 不连接银行、企业系统或其他账户，安装完成后的常规仪表盘不会调用外部 API。只有用户主动运行 `yuxin update` 时会访问 GitHub；主动运行 `yuxin share --anonymous`、检查待上传字段并输入 `SHARE` 后，才会向公开看板的 Supabase 服务提交降精度数据。

匿名贡献不上传配置文件、年龄、生日、性别、姓名、单位、城市或设备标识；月薪按百元、存款按千元、工时按 30 分钟归一化，距离退休只提交整数年。应用数据表不保存 IP 或 User-Agent，但 Supabase 基础设施日志仍可能按网络服务惯例处理 IP、请求时间和 User-Agent，因此这里承诺的是应用层匿名化，而不是网络层不可追踪。

```text
今日入账 = 当日有效工作秒数 × 每秒收入
退休日期 = 出生日期 + 按性别估算的渐进式退休年龄
剩余总年数/总月数 = 剩余天数 ÷ 365.2425 / 30.436875，向下取整
实时存款余额 = 手动填写的存款 + 按当前配置回算的完整工作日工资 + 今日已入账
撑到退休每天可花 = 实时存款余额 ÷ 距退休的日历天数
撑到退休每月可花 = 每天可花 × 30.436875
```

完整工作日工资从存款起算日计至今日之前，使用当前薪资、工作时间和已附带年份的节假日数据回算，不保存每天的历史账本。修改存款金额会从当天重新起算；修改薪资或工作时间后，已有区间也会按新配置重新估算。

退休估算参考[《国务院关于实施渐进式延迟法定退休年龄的决定》](https://www.gov.cn/yaowen/liebiao/202409/content_6974294.htm)。程序不读取身份证或参加工作年份。只填年龄时，以当天作为估算生日；女性默认按原 55 岁口径估算，旧配置中的原 50 岁口径仍兼容。结果不构成政策或财务建议。

## 从源码构建

需要 Go 1.22 或更高版本，项目只使用标准库：

```bash
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w -buildid=" -o yuxin ./cmd/yuxin
./yuxin
```

开发环境、演示素材生成和版本发布流程见[开发者文档](https://github.com/wnma3mz/yuxin/blob/main/docs/development.md)。参与贡献前请阅读[贡献指南](https://github.com/wnma3mz/yuxin/blob/main/.github/CONTRIBUTING.md)，版本变化见 [CHANGELOG.md](https://github.com/wnma3mz/yuxin/blob/main/CHANGELOG.md)。

## License

[MIT](LICENSE)

欢迎参与项目。安全问题请按[安全政策](https://github.com/wnma3mz/yuxin/blob/main/.github/SECURITY.md)私密报告。
