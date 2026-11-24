# 当前需求方案

2026-11-23，基于 SeqAnt 当前 `2e37094`。

| 需求 | 方案入口 | Trellis 状态 |
| --- | --- | --- |
| 公共 MCP 站内可视化配置 | [完整方案包](public-mcp-catalog/README.md) | 方向 / 线框已认可；完整方案待实施确认；父任务 + 六子任务均 planning |
| Codeup 平台集成 | [需求与待选范围](codeup-integration/prd.md)、[设计](codeup-integration/design.md)、[实施计划](codeup-integration/implement.md)、[研究](codeup-integration/research.md) | 已登记独立任务；待选择是否包含云效 OpenAPI / MR |

公共 MCP 已完成首批站内目录、安装权限、HTTP 严格检查和 stdio 隔离检查实现；Codeup 已完成首期个人 PAT、仓库识别与主机隔离实现。真实外部服务、真实 Codeup 凭证、生产部署和远程推送仍需独立验收，不能由本机测试替代。Trellis 使用现有本地开发管理工作区，任务状态不等同于运行验收。
