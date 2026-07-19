# 开发与发布

本文面向项目维护者，集中说明本地验证、演示素材生成和版本发布流程。提交 Pull Request 的约定见[贡献指南](../.github/CONTRIBUTING.md)。

## 环境

- 使用 `go.mod` 声明的 Go 版本或更高版本。
- 项目运行代码只使用 Go 标准库。
- 正式 Release 使用的 Go 版本以 `.github/workflows/build-binaries.yml` 中的 `RELEASE_GO_VERSION` 为准。

## 本地验证

提交代码前运行：

```bash
test -z "$(gofmt -l .)"
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w -buildid=" -o yuxin ./cmd/yuxin
./yuxin --version
```

日常 CI 在 Ubuntu 上执行 Go 单测、覆盖率、Vet、构建和冒烟测试，并验证 Supabase SQL 隐私契约、Web 生产构建与工作流配置。跨平台构建、ARM64、安装器和 Homebrew Formula 只在发版工作流中验证。

## 演示素材

演示数据固定为合成配置，不读取用户配置，也不访问网络；时间、今日入账和实时存款会在画面中按秒重新计算。

生成 README GIF 需要 `agg`。macOS 可安装：

```bash
brew install agg
```

然后运行：

```bash
scripts/render-demo.sh
```

输出文件为 `docs/assets/yuxin-demo.gif`。

生成带声的 1080p 宣传视频还需要支持 `drawtext` 的 FFmpeg。macOS 可安装：

```bash
brew install ffmpeg-full
```

然后运行：

```bash
scripts/render-promo.sh docs/assets/yuxin-promo.mp4

# 生成抖音与 TikTok 共用的中文版竖屏宣传片
scripts/render-social-cn.sh docs/assets/yuxin-social-cn.mp4
```

两个脚本默认混入仓库内的 `docs/assets/yuxin-promo-voice.m4a`。可通过以下环境变量调整：

- `YUXIN_PROMO_SILENT=1`：生成无声横版。
- `YUXIN_PROMO_VOICE_TRACK=/path/to/voice.m4a`：替换横版配音音轨。
- `YUXIN_SOCIAL_VOICE_TRACK=/path/to/voice.m4a`：替换竖屏配音音轨。
- `YUXIN_FFMPEG=/path/to/ffmpeg`：指定 FFmpeg 可执行文件。

演示分镜、画面和配音稿统一见[演示与宣传视频计划](demo-video-plan.md)。

## 发布版本

发布前确认：

1. `internal/app/VERSION` 已更新为目标版本。
2. `CHANGELOG.md` 包含同版本标题和至少一条变更记录。
3. 本地验证通过，相关代码、文档和素材已提交并推送。

推送与版本文件一致的标签：

```bash
version=$(tr -d '[:space:]' < internal/app/VERSION)
git tag "v$version"
git push origin "v$version"
```

`.github/workflows/build-binaries.yml` 会再次测试并构建 macOS、Windows 和 Linux 的 x86_64/ARM64 发布包，同时验证 Shell/PowerShell 安装器和 Homebrew Formula。所有发版门禁通过后才会创建 GitHub Latest Release。Release 说明取自 `CHANGELOG.md` 中的对应版本。

Release 成功后，工作流会使用两个 macOS ZIP 的 SHA-256 自动更新 `wnma3mz/homebrew-tap`。首次启用或调整 Tap 权限时，按照 [Homebrew 发布配置](homebrew.md)操作。

手动运行 `build-standalone-binaries` 工作流只生成构建产物；没有版本标签时不会创建 Release 或更新 Homebrew Tap。

## 相关文档

- [配置与产品决策](config-review.md)
- [演示与宣传视频计划](demo-video-plan.md)
- [产品与发布路线](roadmap.md)
- [Homebrew 发布配置](homebrew.md)
- [版本变化](../CHANGELOG.md)
