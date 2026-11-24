# Codeup 来源与代码研究

日期 2026-11-23；源码 `2e37094`。已取得以下官方页面正文；未取得用户真实 Codeup 凭证，未调用真实账号接口。

## 官方资料

| 来源 | 核对结果 |
| --- | --- |
| [用户指定：代码管理概述](https://help.aliyun.com/zh/yunxiao/user-guide/what-is-code-management) | Codeup 包含托管、评审、检测、搜索与 OpenAPI；产品说明不是可直接复用的 API 契约 |
| [个人访问令牌](https://help.aliyun.com/zh/document_detail/206850.html) | PAT 可用于云效 API 及 HTTP Git；授权范围和有效期需要最小化；只在创建时返回明文 |
| [HTTPS 克隆账号 / 密码](https://help.aliyun.com/zh/yunxiao/user-guide/configure-https-clone-account-password) | 使用 HTTPS 管理仓库可配置克隆账号 / 密码，账号允许修改；不能把阿里云控制台登录密码当成默认要求 |
| [OpenAPI 概览](https://help.aliyun.com/zh/yunxiao/developer-reference/api-devops-2021-06-25-overview) | 所列 devops/2021-06-25 使用 ROA / AccessKey 签名体系；不能据此假设它与 PAT 接口完全一致 |

OpenAPI 的具体版本、PAT 访问路径与鉴权头、Git PAT 用户名约定须按选定范围进一步核对对应官方契约；未验证前不写死 `oauth2`、`PRIVATE-TOKEN` 或 GitLab API 路径。组织标识名与组织 ID 不必相同，Git 路径可用于克隆身份但不冒充 API ID。

## 当前代码链路

| 所有者 | 现状 / 调整面 |
| --- | --- |
| `web/app/workbench.js` 的 CREDENTIAL_KINDS | 平台集成向导由配置表驱动；新增 Codeup 行、说明与输入 |
| `chatbot/src/credentials.js`、`web/app/access.js` | 凭证名称、缺失错误识别、设置跳转需同步 |
| `internal/domain/user.go` 的 PublicUser | 新增敏感字段必须清空明文，只输出配置状态 |
| `internal/repo/users.go` 的 userConfigRow / UpdateUserCredentials / writeUserConfig | 用户配置读写、保留 / 清除和配置存在性判定需同步 |
| `internal/repo/platform_git_credentials.go` 的 GitHostCredentials | 当前仅 GitHub/Gitee/GitLab；需 Codeup 精确主机绑定，缺凭证也登记空值防回退 |
| `internal/gitwork/host_credentials.go` | 现有 host map 与 Git helper 使用环境变量，避免密钥写入命令行或仓库 |
| `internal/domain/public_repository.go` | 当前 provider 只含 GitHub / Gitee / GitLab 与原有兼容解析；新增 Codeup 独立分支保留多层路径 |
| `internal/httpapi/catalog.go`、`web/app/issues.js`、`project-wizard.js`、`projects.js` | 仓库识别 / 提供方 / 凭证提示 / 文案必须同步 |
| `internal/httpapi/git_token_verify.go` | 现有验证明确针对 GitHub，不能把 Codeup Key 发往 GitHub 验证端点 |
| `internal/runtime/command_binding.go`、`internal/engine/run_environment.go`、`runner_dispatch.go`、`internal/runner/daemon.go` | 责任人凭据、运行注入和 Runner 分发已有链路，应复用而不是只改表单 |
| `internal/domain/delivery_policy.go` | 新 Codeup 环境变量必须被受控路径剥离，不能成为绕过限制的别名 |
| `internal/httpapi/assistant_diagnostics.go`、`assistant_mcp.go` | 新密钥需加入已有诊断 / 错误脱敏集合 |

相关既有回归：`webtest/tests/workbench-credentials.test.mjs`、`internal/httpapi/provider_credentials_test.go`、`internal/repo/platform_git_credentials_test.go`、`internal/gitwork/host_credentials_test.go`、各 engine/runtime/runner 的 `platform_git_credentials_test.go`。本轮尚未执行新功能测试。
