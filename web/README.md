# Yuxin 匿名数据看板

> 状态：已部署上线；目标 Supabase migrations、GitHub Pages 与公开 URL 验收均已完成。

Supabase Project ID：`nubeymzysjmlwgzjpstl`；Project URL：`https://nubeymzysjmlwgzjpstl.supabase.co`；区域：Northeast Asia (Tokyo)，`ap-northeast-1`。仓库与前端只使用 publishable key，绝不使用 secret/service-role key。GitHub Pages 使用默认地址 `https://wnma3mz.github.io/yuxin/`。

[打开线上看板](https://wnma3mz.github.io/yuxin/)

这个目录用于独立的公开 GitHub Pages 站点。它允许访客主动、匿名地贡献一组 Yuxin 相关数据，并向所有人展示聚合后的统计看板。它不是个人数据管理工具，不提供个人主页、个人看板或账号体系。CLI 仍保持离线、本地优先；网页不会自动读取本机的 `yuxin.toml` 或 `~/.config/yuxin/config.toml`。

## MVP 边界

第一版只提供两个公共页面状态：

1. **公开看板**：所有访客均可查看样本量、中位数和区间分布。
2. **匿名提交**：用户明确填写、预览并同意后提交数据。

暂不处理用户故意提交虚假数据，但仍会做字段范围校验和数据库权限隔离。

## 永久产品边界

以下能力永远不做，也不进入后续路线图：

- **账号系统**：不注册、不登录，不建立可恢复的匿名身份。
- **个人空间**：不提供个人主页、个人看板、投稿历史或个人档案。
- **排行**：不对工资、存款、工时、退休进度或贡献次数做个人排名。
- **社交关系**：不提供关注、好友、点赞、回复、私信或用户关系链。

匿名回声只是经过审核的公共内容流，不显示作者、不形成身份，也不支持互动。公开站点不建设账号系统；内部数据治理与公开产品完全隔离。

## 推荐技术方案

- 目录：`web/`
- 前端：Vite + TypeScript，不引入 React/Vue
- 图表：原生 SVG/CSS，MVP 不增加图表库
- 数据访问：浏览器原生 `fetch` 调用受限 Supabase RPC，不增加运行时 SDK
- 托管：GitHub Pages，通过独立 GitHub Actions 工作流部署 `web/dist/`
- 数据库：Supabase Postgres + 面向公开访客的受限 RPC
- 数据库变更：SQL migration 纳入 `web/supabase/migrations/`
- 测试：数据计算与表单校验单元测试、Supabase 权限集成测试、生产构建验证

GitHub Pages 只托管静态文件，不存放服务端密钥。浏览器只能使用 Supabase `sb_publishable_...` key；任何 `secret` 或 `service_role` key 都不得进入前端、GitHub Pages 构建产物或仓库。

## 推荐公开提交模型

网页不创建 Supabase 匿名用户，也不生成个人资料。浏览器使用公开的 publishable key，只能调用四个职责固定的数据库函数：

- `submit_public_data`：校验字段范围并新增一条匿名贡献。
- `submit_public_message`：独立提交一条等待审核的匿名回声。
- `get_public_dashboard`：只返回样本量和聚合统计。
- `get_public_messages`：只返回审核通过的匿名一句话。

原始表不向 `anon` 角色授予直接查询、修改或删除权限。四个函数固定 `search_path`，显式限制 `EXECUTE` 权限。公开结果不展示单条数值记录，也不返回样本量过小的交叉分组。

第一版提交是单向的：提交后不会形成“我的数据”，访客也无法凭浏览器身份修改历史记录。提交前会再次确认；页面可以用本地状态记录本机曾经提交，但这不是身份或防伪机制。

### 匿名性的准确边界

- Yuxin 应用数据不保存姓名、联系方式、IP、User-Agent、设备 ID、稳定匿名 ID或精确提交时间。
- 金额与工时在客户端先降精度，数据库函数再次执行相同归一化：月薪按百元、存款按千元、工时按 30 分钟。
- 不上传年龄、生日、出生年月或性别；退休信息只保留整数剩余年数。
- 匿名回声与数值贡献使用独立请求、独立事务和两张表，不设置外键或应用层关联标识；Supabase 请求日志仍可能按请求时间形成基础设施层线索。
- 公开页面永远不返回原始数值记录，只返回聚合统计和审核通过的独立留言。
- Supabase API 日志仍可能包含 IP、国家/地区、User-Agent 和请求时间。因此产品只能承诺“应用层匿名化”，不能宣称网络层完全不可追踪。
- CLI 在用户最终确认前不发起上传请求；本地区间判断也不联网。Web 是在线页面，打开和读取聚合数据本身会产生普通网络请求。

## 建议采集字段

币种第一版固定为人民币，所有字段都会在提交前清晰展示。

| 字段 | 建议 | 用途 |
| --- | --- | --- |
| 月薪 | 必填，按百元归一化 | 月薪区间、中位数、折算时薪 |
| 每日净工作时长 | 必填，已扣午休，按 30 分钟归一化 | 工时分布、折算时薪 |
| 每周工作天数 | 必填，1–7 | 折算时薪 |
| 当前存款 | 选填，按千元归一化 | 存款区间与退休支撑能力 |
| 距离预计退休年数 | 选填，整数 | 退休倒计时分布 |
| 一句话类型 | 选填，建议 / 吐槽 / 许愿 / 打气 | 匿名回声分类 |
| 匿名一句话 | 选填，1–80 个字符 | 分享建议、吐槽、愿望或鼓励 |

除这句明确可选的匿名文本外，应用数据不设置姓名、邮箱、手机号、精确生日、单位、职位、城市、其他自由文本或设备指纹字段。输入区提示用户不要填写姓名、联系方式、单位和地址。数据库只保存提交日期，不保存精确时间；Supabase 基础设施日志的边界按上方说明披露。

### 匿名回声

- 一句话与工资、存款等数值分开提交、分表保存，公开页面不建立二者之间的关联。
- 公开回声只显示「建议 / 吐槽 / 许愿 / 打气」标签和正文，不显示精确提交时间。
- 文本提交后默认为 `pending`；数值最早在下一 UTC 自然日进入候选集，并且只在凑满新的 10 份后整批释放。公开总数按 10 份、分桶和有效样本数按 5 份向下分组，中位数再按固定粒度取整。正文需在 Supabase 后台标记为 `approved` 才会公开。
- 第一版不开发管理后台，由项目所有者直接在 Supabase Studio 中审核。
- 前后端共同限制为单行 80 字，拒绝链接和控制字符；前端始终按纯文本渲染，不解析 HTML 或 Markdown。
- 看板每次只展示少量已审核内容，避免留言墙抢走数据看板的主视角。

## 首版看板

- 公开样本数，以及各选填字段按隐私门槛释放的有效样本数
- 月薪中位数、每日净工作时长中位数，以及「月薪 × 工时」四象限
- 性价比大考与赛博躺平指数共用一个数据舞台，可切换主题，并在象限洞察与左右分布明细之间切换
- 按月薪、净工时和工作天数计算的折算时薪中位数
- 「月薪 × 躺平日均预算」赛博躺平指数，日均预算按现有存款撑到预计退休计算
- 宏观指标采用主次卡片与样本工资脉冲；脉冲按用户本地当前时间、09:00 统一起点、净工时中位数及随包节假日数据估算，不采集个人上下班时间。象限和区间只显示固定趣味旁白，不冒充从用户回声提取的高频词。
- 匿名回声：三行对冲跑马灯随机展示已审核的建议、吐槽、许愿和打气
- 数据口径、更新时间与匿名说明

首版按页面加载和手动刷新取数，不启用 Supabase Realtime。原始记录不公开下载。

## 目录结构

```text
web/
├── index.html
├── package.json
├── package-lock.json
├── tsconfig.json
├── vite.config.ts
├── .env.example
├── public/
│   └── favicon.svg
├── src/
│   ├── main.ts
│   ├── api.ts
│   ├── mock.ts
│   ├── model.ts
│   ├── validation.ts
│   └── styles.css
├── tests/
│   ├── api.test.ts
│   ├── model.test.ts
│   └── validation.test.ts
└── supabase/
    ├── migrations/
    └── tests/
```

实施进度、已确认口径和上线清单见 [Web Roadmap](ROADMAP.md)，页面变更见 [Web Changelog](CHANGELOG.md)。管理员操作手册只保存在本机并由 `.gitignore` 排除。

## 官方参考

- [GitHub Pages 自定义工作流](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages)
- [Supabase 前端数据安全](https://supabase.com/docs/guides/database/secure-data)
- [Supabase Row Level Security](https://supabase.com/docs/guides/database/postgres/row-level-security)
- [Supabase Database Functions](https://supabase.com/docs/guides/database/functions)
- [Supabase API 日志与请求元数据](https://supabase.com/docs/guides/telemetry/logs)
