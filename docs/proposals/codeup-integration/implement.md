# Codeup 实施计划草案

Trellis：`11-23-codeup-platform-integration`；首期 Git-only / 个人 PAT 已完成本机实现与测试。方案基于当前源码和官方资料，不包含真实云效账号验证。

## 依次完成

1. **CU-T1 契约确认**：收敛用户选择的集成深度；核对 Git 认证用户名、PAT / 克隆密码和官方 clone URL。若包括 OpenAPI，补完整鉴权 / endpoint / MR 契约再评审。
2. **CU-T2 凭证全链路**：用户字段 / 存储 / 配置状态 / 更新清除 / PublicUser 脱敏；新增平台列表与输入；诊断与 chatbot 缺凭证定位同步。
3. **CU-T3 仓库与执行**：Codeup URL 识别、provider、命名空间、host helper、责任人 / Runner 分发、缺凭据阻断，避免其他平台密钥回退。
4. **CU-T4 回归与证据**：权限、清除、密钥不泄露、恶意域名、多层路径、本机 / Runner、受控策略；有真实资源时执行读写验收，未提供则如实列外部验收未完成。

修改范围以 [research.md](research.md) 的所有者表为准；不为五个平台建立通用集成框架。

## 计划验证

```bash
git diff --check
go test ./internal/domain ./internal/repo ./internal/gitwork ./internal/httpapi -count=1
go test ./internal/runtime ./internal/engine ./internal/runner -count=1
node --test webtest/tests/workbench-credentials.test.mjs
npm --prefix webtest test
npm --prefix chatbot test
make build
```

已核对 chatbot/package.json，test 脚本为 vitest run；实施时仍需确认实际工具链可用。测试数据使用合成凭据；本地 HTTPS / Git fixtures 验证传递机制，不能声明真实云效认证通过。

## 验收矩阵

| 用例 | 证据 |
| --- | --- |
| 平台入口新增 / 修改 / 删除 / 搜索 / 缺凭据定位 | 实际页面截图和操作记录 |
| 甲用户保存，乙用户不可读写，访客不可写 | 认证 API 正反例 |
| 公开响应、日志、错误、argv / remote / 文件无密钥 | 特征令牌检测 |
| Codeup URL 多层路径、非法片段、仿冒域名、SSH 分流 | parser / host helper 正反例 |
| Codeup 缺凭据时不使用 GitHub/GitLab Key | 已有 runtime / Runner 路径回归 |
| 更新 / 清除不影响其他平台，下一任务使用新配置 | 用户与执行链路测试 |
| 实际仓库克隆 / 工作分支推送 | 真实 Codeup 测试仓库与凭据引用；无资源时未验证 |
| 若选择 MR：准确 source SHA、幂等创建、人审约束 | 独立云效 OpenAPI 与平台交付实验，不能由 Git 推送替代 |

首期已按推荐范围执行；云效 OpenAPI / MR 仍需另立任务和官方契约核对，不在本批自动扩展。
