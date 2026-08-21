# cry-072 场地安全隐患分级升级系统

这是一个离线运行的场地安全巡检与隐患处置项目。系统根据风险评分和版本化规则把隐患放入普通队列或重点跟进队列，并把升级依据、处置、复检、通知和审计事件保存在同一个业务闭环中。

## 模块职责

- `cmd/server`：装配依赖、启动 HTTP 服务和本地通知调度器，并处理优雅停机。
- `internal/domain`：场地、设施、风险规则、隐患、队列、整改、复检和状态机规则。
- `internal/application`：上报、分级、负责人分派、状态流转、整改、复检、通知、统计和审计用例。
- `internal/repository`：内存事务仓储与 pgx/PostgreSQL 持久化实现。
- `internal/service`：风险计算、规则匹配和处置时限策略。
- `internal/transport/http`：`/api/v1` Gin 接口、分页筛选和稳定错误响应。
- `internal/middleware`：request ID、结构化访问日志、panic 恢复、CORS 和安全响应头。
- `internal/platform`：本地附件、通知、时钟和 ID 适配器。
- `migrations`：可重复执行的数据库结构和非覆盖式演示数据。
- `api/openapi`：OpenAPI 3.0 接口契约。
- `web`：Vue 3、TypeScript、Vite 和 Pinia 中文操作界面。

## 本地启动

需要 Go 1.24 或更高版本、Node.js 20 或更高版本。PostgreSQL 16 是持久化运行方式；不设置 `DATABASE_URL` 时服务使用内存仓储并自动装载演示数据，便于完全离线演示。

```powershell
Copy-Item .env.example .env
$env:DATABASE_URL = ''
go run ./cmd/server
```

服务默认监听 `http://localhost:8080`，健康检查为 `/healthz`，仓储就绪检查为 `/readyz`。前端开发服务器单独启动：

```powershell
Set-Location web
npm ci
npm run dev
```

## 配置

| 变量 | 默认值 | 用途 |
|---|---|---|
| `APP_ENV` | `development` | `production` 时使用生产结构化日志 |
| `HTTP_ADDR` | `:8080` | HTTP 监听地址 |
| `DATABASE_URL` | 空 | pgx PostgreSQL 连接串；空值启用内存仓储 |
| `DATABASE_TIMEOUT` | `5s` | 单次数据库 I/O 超时 |
| `SHUTDOWN_TIMEOUT` | `10s` | HTTP 与调度器停机期限 |
| `ATTACHMENT_DIR` | `./runtime/attachments` | 受控本地附件目录 |
| `ATTACHMENT_MAX_BYTES` | `8388608` | 单个附件最大字节数 |
| `DEMO_OPERATOR_ID` | `operator-demo` | 本地演示操作人 |
| `DEMO_OPERATOR_ROLE` | `supervisor` | `admin`、`supervisor`、`reviewer` 或 `inspector` |

不要把真实密码写入 `.env.example` 或提交到 Git。运行期附件、日志、依赖缓存和前端构建产物已由 `.gitignore` 排除。

## PostgreSQL 迁移与演示数据

迁移使用 `IF NOT EXISTS`、稳定唯一约束和 `ON CONFLICT DO NOTHING`，重复执行不会覆盖已有业务数据。

```powershell
./scripts/migrate.ps1 -DatabaseUrl 'postgres://safety:safety@localhost:5432/safety?sslmode=disable' -Seed
$env:DATABASE_URL = 'postgres://safety:safety@localhost:5432/safety?sslmode=disable'
go run ./cmd/server
```

演示数据创建一个滨江场地、一处关键配电设施和一个电气风险分类。应用层的内存演示模式还会装载一条重点升级规则，用于直接体验自动分级。

## 核心状态与规则

隐患状态按以下路径推进：

```text
待确认(pending_confirmation) -> 处理中(in_progress) -> 待复核(awaiting_reinspection)
-> 已解除(resolved) -> 已关闭(closed)
```

- 规则按优先级、版本和条件集合匹配；风险等级、评分、影响范围和关键设施条件必须全部满足。
- 自动升级会保存规则 ID、规则版本和逐项命中依据，并进入重点队列。
- 人工降级需要主管或管理员权限和明确理由；原自动升级依据不会被删除。
- 重点隐患必须有通过的复检记录才能解除或关闭。
- 恢复场地前会检查同场地其他未关闭的重点隐患，避免过早解除隔离。
- 所有更新使用版本号进行乐观并发控制；上报、巡检、复检和通知使用幂等键或去重键。

## API 示例

所有业务接口都位于 `/api/v1`。可通过 `X-Operator-ID` 和 `X-Operator-Role` 覆盖本地演示身份。

```powershell
$body = @{
  idempotency_key = 'report-demo-001'
  site_id = 'site_demo'
  facility_id = 'facility_demo'
  risk_category_id = 'risk_electrical'
  title = '配电柜出现焦糊气味'
  description = '巡检发现柜内温度异常并伴随焦糊气味'
  severity = 5
  likelihood = 4
  impact_scopes = @('人员', '供电')
  initial_action = '现场拉设警戒并安排断电检查'
  evidence = @()
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/hazards `
  -ContentType application/json -Body $body

Invoke-RestMethod 'http://localhost:8080/api/v1/hazard-queues?queue=focus&page=1&page_size=20&sort=due_at'
```

可排序字段限定为 `due_at`、`created_at`、`risk_score` 和 `updated_at`。队列、负责人、状态、风险等级、场地和逾期条件均使用白名单解析。更新接口通过 `If-Match` 传入当前版本，例如 `If-Match: 3`。

错误响应始终包含稳定 `code`、中文 `message` 和 `request_id`，字段错误另含 `field_errors`：

```json
{
  "code": "VALIDATION_ERROR",
  "message": "请求字段不符合业务规则",
  "field_errors": [{"field": "severity", "message": "必须在 1 到 5 之间"}],
  "request_id": "req_123"
}
```

完整接口见 `api/openapi/openapi.yaml`。

## 测试与验证

```powershell
go test ./...
go test -race ./...
go vet ./...
Set-Location web
npm ci
npm test
npm run build
npm audit --audit-level=high
```

本次基线实际验证结果：Go 单元、HTTP、并发和事务测试通过；race 检查通过；`go vet` 通过；前端 2 个测试文件共 3 个用例通过；TypeScript 类型检查与 Vite 生产构建通过；npm 高危依赖审计为 0。PostgreSQL 集成测试在设置 `TEST_DATABASE_URL` 后运行，并使用独立测试数据库验证乐观并发与 JSON 往返。

## 附件与离线边界

附件只写入配置的本地根目录，文件名会被重写为随机存储键，并限制为 PNG、JPEG、PDF 或纯文本及配置的最大体积。通知由本地内存适配器记录，后台每分钟处理到期提醒和再次升级；项目不请求第三方 API、CDN、云存储、在线模型或真实消息服务。
