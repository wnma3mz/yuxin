# 贡献指南

感谢你帮助改进 Yuxin。提交问题前，请先搜索已有 Issue，避免重复。

## 本地开发

项目需要 Go 1.22 或更高版本，并且只使用 Go 标准库。

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./...
```

## 提交 Pull Request

- 每个 Pull Request 只处理一个明确问题。
- 保持实现简单，不引入第三方 Go 模块。
- 新增或修复行为时补充相应测试。
- 面向用户的变更需要同步更新 README 或 CHANGELOG。
- 提交前确保上述命令全部通过。
