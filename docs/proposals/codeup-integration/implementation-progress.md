# Codeup 实施进度

日期：2026-11-24；实现基线：`2e370940`。

## 已落地

- 个人配置增加 Codeup 平台入口，保存 / 替换 / 清除个人 Codeup PAT；公开用户响应只返回 `codeupTokenConfigured`，不会回填明文。
- Git 主机凭证按 `codeup.aliyun.com` 精确匹配，未配置 Codeup 时不会回退 GitHub、Gitee 或 GitLab 令牌。
- 仓库解析支持 Codeup HTTPS、`codeup:组织/路径/仓库` 以及任意合法多层路径；拒绝仿冒域名、用户名密码 URL、路径穿越和不完整路径。
- 责任人凭证通过既有本机 / Runner Git 注入链路传递，令牌不会写入 remote、argv 或仓库文件。
- 迁移脚本为 `migrations/035_codeup_platform_credentials.sql`，生产环境需按仓库迁移规则手动执行；`Open` 的 AutoMigrate 仅负责新增列。

## 本轮验证

- `go test ./internal/domain ./internal/repo ./internal/httpapi -run 'TestPlatformRepositoryURLs|TestPlatformGitCredentialsRemainSeparate|TestMCPMarket' -count=1`
- `node --test webtest/tests/projects-repos.test.mjs webtest/tests/workbench-credentials.test.mjs webtest/tests/access-guides.test.mjs`
- `git diff --check`

## 尚未宣称完成

- 未使用真实 Codeup 账号、仓库或令牌执行克隆 / 推送，因此真实云效认证、权限和网络可达性仍未验证。
- 本批只做 HTTPS Git / 个人 PAT 接入；云效 OpenAPI、组织 ID、合并请求创建 / 审批 / 合并没有实现，也没有把 Git 推送当作 MR 验收。
- HTTPS 克隆账号 / 密码与 PAT 的产品差异已在界面说明；单独的克隆账号字段留待后续范围确认，不与 PAT 混用。
