# Controlled delivery deployment acceptance

Controlled execution creates a draft PR only. It never merges, enables auto-merge or equates execution success with business acceptance. Hosting must enforce review independently of platform prompts and API permissions.

## Hosting policy

Run `python3 scripts/check-delivery-policy.py owner/repository` with an authenticated GitHub CLI. It reads the actual default branch, effective branch rules and contributing repository rulesets. Acceptance requires one or more approvals, stale-review dismissal, a different approver after the last push, resolved discussions, no force-push/deletion, active rules and no bypass actors. The script does not modify hosting settings.

A ruleset matching `~DEFAULT_BRANCH` should apply these rules with `enforcement: active` and an empty `bypass_actors` list. Changes to approval requirements are an operator action, separate from publication. Protect the repository that receives contributions; protecting only the platform source repository does not protect other project targets. The PR author cannot supply their own required approval. Conversion from draft and human approval remain explicit human actions.

## Publisher identity

Use a repository-selected GitHub App installation or fine-grained token. Give only metadata read, contents read/write for create-only integration branch/object publication and pull requests read/write for draft PR creation. Do not give administration, Actions/workflows, organization administration or rule bypass rights. GitHub contents/PR write permissions alone are not a create-only transport: effective branch rules must prevent default-branch writes and unreviewed merging.

Keep publishing credentials only in the controller's protected user/provider configuration. Workers receive neither hosting credentials nor memory/configuration publication rights. A personal Worker can still hold ambient host credentials; use a clean execution account and an appropriate containment profile.

`--require-least-privilege` deliberately exits unsuccessfully because a read-only ruleset inspection cannot prove the deployed token's scope. Operators must record the constrained installation/repository selection and effective permissions, and test denied default-branch writes, force-push, bypass and unreviewed merge in a disposable repository. A broad existing `gh` operator session must never be reported as that constrained identity. Do not place secrets in the evidence or shell arguments.

## Database and runtime

Existing installations review and manually apply numbered SQL migrations. There is no startup data migration or seed. Build the chatbot (`bash scripts/build-chatbot.sh`) before building the Go server, which embeds `web/native-chat`. Deploy current platform and compatible Runner binaries together. Node placement requires Runner 0.11.0+ and attempt-isolated repositories with explicit base commits. Governed release/memory execution additionally requires Runner 0.12.0+; old Runners cannot claim those tasks.

Local Mac/Docker Linux evidence proves cross-OS execution on one physical host. Independent cloud-host network/TLS/recovery, production artifact retention, and hostile-code containment require deployment-specific acceptance.
