# Gitee / GitLab 个人凭证

在「个人配置 → 添加凭证 → 平台集成」选择 **Gitee** 或 **GitLab**，按向导创建并填写个人访问令牌。已配置的凭证可在同页展开修改或删除；保存后不再返回令牌明文。

- Gitee：适用于 `gitee.com` 的 HTTPS 仓库，在 [私人令牌](https://gitee.com/profile/personal_access_tokens) 页面创建令牌并授予目标仓库的读写权限。
- GitLab：适用于 `gitlab.com` 的 HTTPS 仓库，在 [Access Tokens](https://gitlab.com/-/user_settings/personal_access_tokens) 页面创建令牌，勾选 `read_repository` / `write_repository`。自建 GitLab 使用现有 Git SSH 主机密钥入口。
- GitHub 的授权、令牌校验与 SSH 密钥管理保持原有入口。Gitee/GitLab 采用手动令牌配置，不调用 GitHub 的 OAuth 或校验接口；「已配置」只表示已保存，不表示远端授权已验证。

任务仍使用需求责任人（未指定时为项目 Owner）的凭证。HTTPS 检出、Runner 仓库绑定和 Git 子进程按主机选择令牌；未配置的平台不会回退到另一个平台的令牌。受控交付策略继续移除仓库写凭证，不因新增平台开放自动合并或直推。

本次增加的是凭证管理与 Git HTTPS 认证，不包含 Gitee/GitLab OAuth、仓库选择器、Issue 同步或自动创建 PR/MR 的 API 适配；现有 GitHub 专用工具仍只操作 GitHub。

项目仓库表单支持粘贴 Gitee/GitLab HTTPS 地址，GitLab 支持多层子分组，并将 `/-/tree/...` 页面地址归一为仓库地址。

## 存储与部署

令牌沿用个人凭证存储方式，分别保存在 `user_configs.gitee_token` / `gitlab_token`。公共用户响应只返回 `giteeTokenConfigured` / `gitlabTokenConfigured`，不会返回令牌。登录刷新和其他凭证的修改不会覆盖它们。

`migrations/034_platform_git_credentials.sql` 是手动 DDL 脚本；启动时现有增量 DDL 也会补充缺失列。已由增量 DDL 创建列的数据库不要重复执行脚本。

更新服务需重新构建前端资源与服务端，并同步更新 Runner，以使用按主机匹配凭证的子进程绑定。

## 验证范围

本地测试覆盖凭证新增/修改/删除、登录刷新保留、API 所有者权限与脱敏，以及真实 `git credential fill` 对多平台凭证的选择、缺失凭证拒绝回退和受控 Runner 任务的凭证清除。浏览器界面验收使用隔离模拟 API；真实 Gitee/GitLab 私有仓库的授权与推送需要有效令牌另行验收。

2026-11-23：相关功能的定向 Go 测试（8 个包）、27 项原生界面测试、368 项 chatbot 测试、chatbot 构建和服务端/Runner 编译通过。扩大 Go 回归时仍有既有失败；已用 `50d7867` 的原始 Go 源码覆盖层复现运行时、Runner 和受控交付相关失败，不能将本轮结果作为全量回归通过或生产部署验收。
