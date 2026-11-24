# 公共 MCP 代码与来源研究

核查日期 2026-11-23；源码基线 `2e37094063b4a4f25b804427c256f27b1007a752`。下列为当前源码事实，不是已经完成的新功能。

## 1. 当前所有者与复用点

| 证据位置 | 事实 | 规划影响 |
| --- | --- | --- |
| `web/index.html:689` | 主按钮跳转 GitHub servers | 替换为站内动作 |
| `web/app/mcp.js:5` | 已加载 `/api/mcp-market` 供模板 / 别名使用 | 目录继续共用，不建新市场 |
| `web/app/mcp.js:38` | 旧目录安装固定为项目范围 | 扩展参数与个人范围 |
| `web/app/mcp.js:47` | 个人筛选排除 `marketId` | 必须取消该排除条件 |
| `web/app/mcp.js:136` | 已有自定义表单 | 共用归一化 / 脱敏规则，保留高级入口 |
| `web/app/mcp.js:194` | 目录项隐藏修改按钮 | 新增目录参数编辑入口 |
| `internal/httpapi/mcp_market_catalog.json` | 仅 GitHub 一项 | 其他四项都是候选 |
| `internal/httpapi/mcp_market.go:25` | 固定目录结构，缺字段声明和模板版本 | 最小扩展结构，不做通用表单平台 |
| `internal/httpapi/mcp_market.go:80` | 先列表去重，再调用通用写入 | 存在先查后写竞态 |
| `internal/repo/catalog.go:176` | `PutMCPServer` 使用 upsert | 目录首次安装需独立 create-if-absent，不能覆盖并发安装 |
| `internal/repo/models.go:177` | ID 主键 + JSON payload | 元信息可增量写入 payload，首期不需要新增表 |
| `internal/domain/assets.go:353`、`:370` | 个人范围禁止 `MarketID` | 前端和安装 API 改动之外还必须调整领域规则 |
| `internal/domain/mcp_test.go` | 现有测试明确要求拒绝个人目录项 | 按新的授权边界更新正反例，不能只删测试 |
| `internal/httpapi/mcp.go:55` | 已有市场项禁止直接修改 | 使用专属模板编辑接口，保留作用域和引用 |
| `internal/httpapi/project_write_access.go:177` | 安装 / 自定义已有按范围写权限检查 | 新检查 / 编辑路由也必须接入，不能遗漏 |
| `internal/domain/mcp_masking.go` | Header 全脱敏、敏感 env 脱敏，掩码可恢复 | 复用；不宣称已加密存储 |
| `internal/mcpclient/client.go:208`、`:267` | tools/list 支持 context，但初始化未绑定调用 context，部分失败被忽略 | 不能直接用返回值作为严格检查通过；补严格检查路径并做兼容回归 |
| `internal/runtime/mcp_call.go:578`、`:600` | 已有 stdio tools/list 和 RPC 初始化；内部直接启动进程 | 只能在隔离沙箱内复用；不能由 handler 在主机直接调用 |
| `internal/runtime/sandbox_manager.go` | 有独立工作区、沙箱生命周期及 Close | 复用专用临时 Docker 沙箱，独立资源清理 |
| `internal/domain/delivery_policy.go:33` | 受控路径移除外部 MCP 配置 | 禁止通过本需求绕过 |

## 2. 公开资料与待核验项

| 来源 | 当前用途 | 尚未证明 |
| --- | --- | --- |
| [MCP servers](https://github.com/modelcontextprotocol/servers) | 用户现有外链；参考实现目录 | 不代表本平台可直接安装 |
| [官方 Registry API](https://registry.modelcontextprotocol.io/v0.1/servers?limit=3) | 前期读取样本，存在版本重复、游标和远端描述 | 全量映射、参数 / 认证适配、兼容缓存 |
| [GitHub MCP](https://github.com/github/github-mcp-server) | 已有目录项来源 | 本轮个人安装与严格检查 |
| [Context7](https://github.com/upstash/context7) | HTTP 候选 | Key 规则、匿名限制、实际工具发现 |
| [Filesystem](https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem) | stdio 首选验证候选 | 固定版本、依赖与隔离运行 |
| [Fetch](https://github.com/modelcontextprotocol/servers/tree/main/src/fetch) | stdio 候选 | Python / uvx 运行依赖和版本 |
| [Playwright MCP](https://github.com/microsoft/playwright-mcp) | stdio 候选 | 浏览器依赖与沙箱支持 |

实施时记录各候选的来源 URL、提交或版本、许可路径、参数、依赖镜像和真实结果。禁止在此提前填造包版本、工具清单或可用性。

## 3. Trellis 使用依据

已实查本机 Trellis CLI `0.6.12`、现有管理工作区的 `workflow.md`、`trellis-brainstorm/SKILL.md` 和 `planning-contract.md`。通过实际 `task.py create --help` 确认 `--description`、`--parent`、`--meta`、`--no-start`，通过 `task.py validate` 检查 JSONL。

当前产品仓没有 `.trellis/`；复用既有管理工作区登记父 / 子任务。不迁移历史项目、不复制部署内容、不覆盖本仓 AGENTS.md。在线 Context7 查询本次返回网络错误，Trellis 命令契约依据已安装源码和本地帮助核对，未据此宣称在线文档已验证。

## 4. 取舍

1. 本地精选目录满足首期目标，避免把全量发现系统和安装适配混在一起。
2. 只支持小范围字段类型；不引入 JSON Schema 运行时或独立 UI 框架。
3. 连接状态保留在本次页面会话，省去分布式检查凭证 / 健康数据库；以后若要历史审计另立需求。
4. stdio 首期验证平台 Docker；独立 Runner 不增加协议，未支持则如实不可测。
5. 增加独立原子安装方法而非改写全局 upsert，减少对已有自定义资产的影响。

## 5. 尚待实施验证

- 官方候选的最终版本、认证字段和工具列表。
- 生产可用 Docker 沙箱的镜像、资源上限和清理行为；本轮不宣称已通过。
- HTTP 严格初始化与旧网关兼容关系；保持旧调用契约并用独立严格探测入口验证。
- 更新后的 UI 与真实 API 联调、实际角色引用和 MySQL 并发安装证据。
