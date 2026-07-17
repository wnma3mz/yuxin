# Homebrew 发布配置

Yuxin 的正式版本仍由 `.github/workflows/build-binaries.yml` 创建。Release 成功后，同一工作流会根据 macOS ARM64 和 x86_64 ZIP 的本地 SHA-256 文件生成 `Formula/yuxin.rb`，并推送到独立 Tap。

## 一次性配置

1. 创建公开仓库 `wnma3mz/homebrew-tap`，默认分支使用 `main`，至少提交一个 `README.md`。
2. 创建仅能访问 `wnma3mz/homebrew-tap` 的 fine-grained personal access token，仓库权限只开启 `Contents: Read and write`。
3. 在 `wnma3mz/yuxin` 的 Actions secrets 中新增 `HOMEBREW_TAP_TOKEN`，值为上一步的 token。

完成后无需手动维护 Formula。推送与 `internal/app/VERSION` 一致的 `v*` 标签时，工作流会：

1. 构建并发布所有平台 ZIP 和 `.sha256`。
2. 使用 `scripts/generate-homebrew-formula.sh` 生成两个 macOS 架构的 Formula。
3. 提交并推送 `homebrew-tap/Formula/yuxin.rb`。

用户安装命令：

```bash
brew install wnma3mz/tap/yuxin
```

本地验证生成器：

```bash
scripts/generate-homebrew-formula.sh VERSION ARM64_SHA256 X86_64_SHA256 /tmp/Formula/yuxin.rb
brew style /tmp/Formula/yuxin.rb
```
