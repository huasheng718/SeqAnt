# Codeup 集成设计草案

状态：首期 Git-only / 个人 PAT 已实现；本文件的 OpenAPI / MR 部分仍是后续设计。需求见 [prd.md](prd.md)，已核实的证据见 [research.md](research.md)。

## 推荐首期设计

1. 延续“项目设置入口、个人凭证所有权”的模型；UsersRepo 管理新字段，usersHandlers 处理本人凭证更新。UI 不新增项目共享凭证概念。
2. 在现有用户配置结构中增加 Codeup 认证值及必要账号字段。PAT 与克隆密码的输入文案 / 字段映射依据官方确认的认证方式，不复用 gitToken / gitlabToken 槽位。
3. 公共 User 返回账号所需非敏感描述及 configured 布尔值，移除令牌 / 密码。更新接口仅接受本人，空白保留、显式 Clear 清除，保存不顺带校验 GitHub。
4. 以 `codeup.aliyun.com` 的 HTTPS 仓库作为首个待验主机，实施时核对官方实际 clone URL；其他地域 / 历史域名不按后缀或通配符自动接纳。主机精确匹配，路径保留组织和任意合法组层级。
5. provider 明确为 codeup；仓库标识与 GitHub / Gitee / GitLab 的同名路径隔离。组织名只影响 Git 路径，不提前要求 OpenAPI 的组织 ID。
6. Git 凭据经现有 host-specific helper 在需要的运行路径注入；令牌不进入 URL、argv、Git remote 或项目配置文件。Codeup 未配置时不回退 GitHub。SSH 继续既有主机密钥链路，HTTPS 凭证不参与。
7. 本机、平台沙箱、Runner 都使用既有责任人身份规则。新变量 / 字段加入诊断、事件和受控模式脱敏 / 剥离逻辑；不变更人审规则。
8. 验证优先针对用户指定仓库执行只读 Git 检查，区分 saved / readable / write-unverified。若后续支持验证 PAT 身份，需先明确官方接口，不能拿仓库公开可读或 200 响应证明令牌有效。

## 数据与兼容

为 user_configs 增加必要的可空字段，默认无配置；遵守 repo.Open 的增量 AutoMigrate 约定，不做启动补数据 / seed。如部署基线需要人工 SQL，在 `migrations/` 中编号新增，不改写旧迁移。

旧用户凭证、公开 User 响应和既有 host map 保持兼容。删除 Codeup 配置不删除其他平台令牌。运行中任务沿用已有快照生命周期，新任务读取新配置；不要承诺删除后已经分发的密钥能被瞬时远程撤回。

## 完整 OpenAPI / MR 方案的额外范围

若用户选择同时做 MR 交付，需要在现有交付所有者下新增独立 Codeup 托管适配：

- 明确 API 版本、PAT 或 RAM / ROA 认证体系、官方 endpoint 及最小权限。
- 组织 ID、仓库 ID、分支与合并请求 ID 的稳定映射，不能从页面 URL 臆造。
- 创建 / 查询 MR 的幂等行为、source SHA / target branch 一致性、错误与限流处理。
- 人工评审与合并边界；不得自动审批或合并。
- 新 publisher 凭据不能直接作为任意 Worker 的管理凭据；真实云效仓库的允许和拒绝操作需分别验收。

这些内容尚未获范围选择，不加入推荐首期的完成声明。

## 发布与回滚

本轮不修改产品代码或部署。后续实现以新增可选字段和 provider 分支为主；回滚不能删除用户密钥。已有 Codeup 绑定仓库在旧版本不识别时应明确提示不支持，不能降级识别为工蜂或 GitHub 并发送错误凭据。
