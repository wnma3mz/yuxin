# Archived demo renderers

余薪的正式 README 动画由 `scripts/render-demo.sh` 使用 asciinema 和 agg 生成。本目录保留两套早期渲染方案，仅供后续视觉设计和技术实现参考。

- `browser/`：Chrome Canvas 渲染 PNG，再由 FFmpeg 合成 MP4 和 GIF。
- `native-macos/`：Go 调用 macOS CoreGraphics/CoreText 直接生成 PNG，并由 Go 标准库合成 GIF。

归档方案不属于日常发布流程，但仍可独立运行：

```sh
./scripts/archive/demo-renderers/browser/render.sh
go run scripts/archive/demo-renderers/native-macos/render.go /tmp/yuxin-demo-native
```

对应样片默认写入系统临时目录，不再保存在仓库中；浏览器渲染脚本也可接收自定义输出目录作为第一个参数。
