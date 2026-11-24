# MCP 实施进度

日期：2026-11-24；当前代码基线起点：`2e37094063b4a4f25b804427c256f27b1007a752`。

## 已落地

- `web/index.html` 的“浏览公共 MCP”已从 GitHub 外链改为站内选择器入口；自定义 MCP 入口保留。
- 公共目录快照扩展为 5 项：GitHub 标记 `ready`；Context7、Filesystem、Fetch、Playwright 标记 `candidate`。候选项可查看来源与依赖，但服务端拒绝直接安装。
- 目录卡片支持搜索、HTTP / stdio 筛选、范围切换、详情和官方来源链接；候选与可添加状态区分。
- GitHub 已接入“详情 → 配置 → 测试连接 → 添加配置”流程。配置页面显示测试位置和受控策略提示；测试只请求 initialize、initialized、tools/list。
- `MCPServerDefinition` 保存模板版本和配置 revision 字段；个人目录安装允许本人拥有，列表和角色资产可见性同步修复。
- 目录安装使用 `CatalogRepo.CreateMCPServerIfAbsent`，避免并发首次安装通过通用 upsert 覆盖第一次配置。
- 已添加公共 MCP 可通过 `PUT /api/mcp-market/installations/:id` 按 `expectedRevision` 修改名称 / 超时；行锁和单调 `configurationRevision` 拒绝过期编辑。
- 新增严格 HTTP 探测客户端和 `POST /api/mcp-market/check`；初始化失败、JSON-RPC 错误、HTTP 非 2xx 不会显示通过。现在支持 MCP Session ID、tools/list 分页（20 页 / 1000 工具上限）和 1 MiB 响应上限。
- 新增 stdio 沙箱探测器：只通过固定 Node Docker sandbox 发送 initialize / tools/list；Docker 不可用、候选未 ready 或没有安装实例时明确返回 unavailable，不回退 API 宿主机。请求有 20 秒边界，沙箱为一次性临时实例。
- 新增前端选择器用例、严格客户端用例、HTTP 检查用例和 Repo 不覆盖用例。
- 已添加安装实例 revision 更新的 Repo / handler 正反例，前端已提供“修改配置”入口。

## 当前验证

通过：

```text
go test ./internal/mcpclient -run 'TestProbeTools' -count=1
go test ./internal/httpapi -run 'TestMCPMarketCheck|TestMCPMarket' -count=1
go test ./internal/domain -run 'TestMCPMarket|TestValidateMCPServerScope' -count=1
node --test tests/mcp-public-selector.test.mjs tests/mcp-catalog-boot.test.mjs tests/mcp-sops-runners.test.mjs
node --check web/app/mcp.js
node --check web/app.js
git diff --check
```

相关前端 8 项用例全部通过。此前尝试的 Go 相关包串行全量命令被仓库既有失败阻断：`internal/repo` 出现并发 SQLite `assistant_diagnostics` 缺表，`internal/runtime` 出现既有 Docker / 环境脱敏 / Git index lock 失败，`internal/httpapi` 全量运行超过 100 秒后手动停止。它们不是本次新增用例的通过证据，也未将全量命令标为通过。

## 尚未完成

- 模板字段参数生成、目录项专属编辑和配置 revision CAS 尚未完成。
- 通用 stdio Docker 握手已用本机 `node:22-bookworm-slim` 临时容器成功发现 `probe_read`，并验证清理路径；当前不能据此宣称 Filesystem / Fetch / Playwright 候选可运行。候选依赖、资源限制和真实发布证据仍待专门环境验收。
- HTTP 探测已接入分页、Session ID 和输出上限；真实外部服务、重定向策略和协议版本兼容仍待环境验收。
- 尚未执行真实外部服务、真实个人凭证、专用 Runner、生产部署或提交推送。
- Codeup 首期 Git-only / 个人 PAT 已实现；云效 OpenAPI / MR 范围未被纳入。
