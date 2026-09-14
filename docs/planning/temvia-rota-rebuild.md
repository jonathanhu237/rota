# 基于模板重新开发 Rota：讨论记录

状态：用户已通过 implement-loop 明确批准实施；采用本地 Markdown 流程，不使用 Trellis。两次修复后，第三轮直接验证发现普通员工角色映射冲突；用户已批准调整，明确普通员工有提交本人空闲时间的权限。主代理已完成直接修复、Centaurus 验证与第三轮累计审查；Standards / Spec 均通过，待提交后用户验收。
固定实施与审查基线：`290be51287caa3831a3b8a14e196664964a2802d`。
分支：`temvia-rota-rebuild`（从本地 `main` 创建；按用户要求不使用 `planning/` 前缀）。

## 已知意图与边界

- 用户希望基于最近完成的模板项目重新开发 Rota。
- 使用 grilling-with-docs 逐轮澄清并记录细节。
- 讨论阶段已结束；用户已批准以 implement-loop 实施、自测和审查，通过后提交。
- 模板来源最终确认为 npm 发布的 Temvia，不采用本地 `../temvia` 源码生成。

## 环境与流程事实

- 创建分支前工作区干净，原分支为 `main`。
- 用户已明确取消 Trellis，根 `AGENTS.md` 已更新为本地 Markdown 流程；不初始化 Trellis，不使用旧 OpenSpec。
- 已有 ADR 位于 `docs/adr/`；后续决定需检查是否冲突。

## 第一轮：已确认

1. 首要目标是降低维护成本；具体成功标准仍待明确。
2. 现有功能必须保留，并融合到模板中；不采用逐项重新批准功能的缩减策略。
3. 真实用户、历史数据迁移、连续可用性和上线时间约束暂不讨论。这不代表授权删除数据或确认这些约束不存在。

## 第二轮：已确认

1. 不再拆解维护成本优先级；模板已开发完成，直接用模板生成底座，不讨论重新设计模板或建立持续上游同步机制。
2. 全部现有功能保留，本次工作是迁移到模板，而非产品重设计或功能裁剪。
3. 此回答确定了迁移方向，不构成开始实施的授权。具体页面/API 兼容边界仍不能从“全部功能保留”推断。

## 第三轮：已确认

1. 使用 npm 发布的 Temvia 生成底座，不使用本地模板 checkout。具体发布版本需在生成前核实并记录，本地调查不代表该发布版本的全部能力。
2. 通用能力以模板为准，Rota 业务接入其中，不保留两套通用实现。
3. 允许按模板调整页面样式、导航结构及 API 路径；完整保留业务功能、业务规则、权限效果和操作能力。

## 第四轮：已确认

1. 以本分支分出时 main 的实际功能作为验收基线，建立逐项功能对照表；代码、文档和测试冲突时列出差异交用户确认，不直接复制疑似 bug。
2. 保留当前仓库与 Git 历史，在临时空目录通过 npm 生成模板后引入本分支，逐步迁入业务并清理被替代的旧实现。

## 后续讨论边界

- 不重新设计现有业务规则；角色权限接入不得无意扩大或缩小现有业务权限。具体映射在功能对照时核实，若与模板冲突再交用户决定。
- npm 精确版本、旧功能清单与验证方案属于下一步事实调查及实施规划，不凭本地模板调查推断发布包内容。
- 实施授权已取得。复杂工作的设计、功能对照表、验收标准与检查清单须在产品实现之前补齐；不得改变已确认范围。

后续基于这些边界核对功能差异；不再询问已确认的迁移动机与功能去留。

## 仓库调查（只读，非已接受决定）

- 本地候选模板实际名为 Temvia，CLI 为 `create-temvia`；仍需用户确认目标仓库。调查时为 `main` / `4ed83b8799a5bae707bee54ae6584481072264b6`。Git 描述为 `v0.3.0-4-g4ed83b8`，package.json 为 `0.2.0`，不等同于 npm 最新发布版。
- Temvia 生成独立源码，没有自动上游升级机制；需要空目录，不能直接覆盖当前 Rota。依据：`../temvia/src/generate.ts`、`../temvia/README.md`。
- 模板已有认证、邀请、细粒度角色权限、账户生命周期、操作历史、品牌、SMTP 配置及异步邮件任务；可考虑替代 Rota 对应通用能力，但兼容规则尚未决定。
- 主要差异：Rota 整数用户 ID / IsAdmin / bcrypt 对比模板 UUID / RBAC / Argon2id；PostgreSQL 17 / database/sql / Goose 对比模板 PostgreSQL 18 / pgx / 独立 up-down 迁移；后端分层和前端组件体系亦不同。依据：两仓库依赖清单、数据库迁移、认证模块。
- 待核对的功能清单包括：岗位与资格、排班模板和需求、发布生命周期、可用时间、自动及手动分配、班表与导出、班次调整及请假代班、考勤与加班、通知和邮件。依据：`backend/cmd/server/main.go`、`backend/internal/service/`、`backend/internal/model/publication.go`。不是仅迁移排班页面。
- 模板要求新增业务模块在账户删除后保留历史关联；需讨论未来业务语义，不等同于现在要做旧数据迁移。依据：`../temvia/template/README.md`。
- 现有 ADR 0001–0003 的 Postgres 优先、按读模型缓存、暂缓 Redis 的核心方向与模板兼容；ADR 0003 的进程内限流与模板 Postgres 限流存在差异，采纳时需明确更新决策。
- 调查时 Rota 无根 `CONTEXT.md`；Trellis 缺失问题已由用户决定取消该依赖而解决。


## 第三轮验证补充确认（不裁剪已确认功能）

模板要求账号至少一个角色、每个角色至少一个权限；当前 Rota 的 `rota.read` / `rota.manage` 都含管理能力，不能用它们给普通员工凑有效角色。角色为空或空权限角色会被模板拒绝登录。用户批准调整，并明确普通员工可提交空闲时间。采用 `rota.self` 作为有效的纯员工角色能力，包含本人空闲时间提交，不授予任何管理读取/写入能力；保留已确认的其他认证后自助能力及所有归属、资格和生命周期校验。不放宽模板非空角色/权限规则，也不另行要求已有有效角色必须追加此权限才能使用原有自助操作。

验证沿用已确认的服务/HTTP/数据库/浏览器验收界面：纯员工角色可通过模板认证并提交本人空闲时间；不能读取管理数据或管理他人排班；真实邮件链接与账户切换验收继续执行。

## 文档记录方式

- 本文件保存讨论进度、待决问题和已确认的需求；建议不视为已接受决定。
- 领域术语明确后即时写入根目录 `CONTEXT.md`，不放入技术设计。
- 重要且难逆转的权衡决定经确认后写入 `docs/adr/`。
- 执行循环：新的 luna-max 子代理按 implement-without-review 实施与自测，主代理直接按 Standards / Spec 两轴审查，最多三轮；同一问题两次修复后仍存在则主代理直接修复并验证。全部通过后仅提交本任务相关变更。
- 审查始终使用上述固定基线及本记录已确认范围，覆盖累计工作区变更和新增文件。
- 初始审查发现：无；各问题修复次数：无。Centaurus 不可用、npm 模板不可用或重大行为冲突时记录证据并停止，不擅自降级或删减功能。

## 实施前事实记录（2026-09-13）

- 已按要求从 npm registry 核实并固定 `create-temvia@0.5.0`，未使用本地 `../temvia` checkout。精确 tarball 为 `https://registry.npmjs.org/create-temvia/-/create-temvia-0.5.0.tgz`；SHA-512 integrity 为 `sha512-EqplgnDgU28sH/mv2nNVZm+/vT5UwBCDDKvLk4M9DvLK8Zmzqe/dqhP8zeniHRpBXhaDY3VhFl1foufC+xSQeA==`，SHA-256 为 `002a2a4aa54cb0f31d5d6e262d88d00055e100df00c79ec44cff0bd82050999e`，npm registry signature key id 为 `SHA256:DhQ8wR5APBvFHLF/+Tc+AYvPOdTpcIDqOhxsBHRwC7U`。
- 使用 `npx --yes --package=create-temvia@0.5.0 create-temvia <empty-dir> --module github.com/jonathanhu237/rota/api` 在临时空目录生成并检查底座；生成目录为 `/tmp/rota-temvia-npm-base-20260913-r1.lMwv6d`。
- 已验证 SSH `centaurus` 连通（经 `centaurus-frp` ProxyJump），远端有 `rsync`、Docker Compose、mise 管理的 Go 1.27.0/Node 24.x/pnpm 11.24.0；已建立权限 `700` 的空隔离验证目录 `/home/jonathanhu237/rota-temvia-rebuild-20260913-r1`，未触碰已有项目或数据。
- 实施设计、清单及完整功能对照/验收矩阵已记录在 [temvia-rota-rebuild-design.md](temvia-rota-rebuild-design.md) 和 [temvia-rota-rebuild-parity.md](temvia-rota-rebuild-parity.md)。
