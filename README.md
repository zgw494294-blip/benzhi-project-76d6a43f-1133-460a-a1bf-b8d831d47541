# 馆藏微环境调控启用台

用于文物保护团队对库房微环境调控方案进行试运行、复核与启用。服务提供建档、基线锁定、方案试运行、观测评估、整改、独立复核、许可签发与验证接口。

## 构建与运行

标准构建：`go build ./...`

运行服务：`go run ./cmd/chamberd -addr=127.0.0.1:19081`

也可以使用 `PORT` 环境变量指定端口号。`-addr` 优先级更高，默认监听 `127.0.0.1:19081`。

可通过 `-data-dir` 或 `CHAMBER_DATA_DIR` 指定档案数据目录，命令行参数优先。

完整自检：`go run ./cmd/chamberd -selfcheck -selfcheck-timeout=8s -addr=127.0.0.1:19081`

测试：`go test ./...`

## 主要接口

- `POST /api/v1/commissioning-cases` 建档；`PATCH /api/v1/commissioning-cases/{caseId}` 修订 Draft 基础信息。
- `POST /api/v1/commissioning-cases/{caseId}/baseline` 锁定基线；`POST .../baseline/revoke` 受控撤销。
- `POST /api/v1/commissioning-cases/{caseId}/plan` 提交方案；`PATCH .../plan` 在试运行前修订方案。
- `POST .../start` 开始试运行；`POST .../observations` 兼容追加单条观测，`POST .../observations/batch` 以 `{"observations":[...]}` 原子登记并评估整批观测；`POST .../remediation` 提交目标偏差及复测观测。
- `GET .../observations/summary` 查询有效观测的数量、温湿度统计、偏差和试运行进度，可用 `from`、`to`（RFC3339）限定窗口。
- `GET .../deviations` 查询偏差台账，支持 `status`、`ruleCode`、`severity`、`observedAtFrom`、`observedAtTo`、`page` 和 `pageSize`，并返回状态计数及每项是否仍可整改。
- `GET .../review-package` 获取带确定性指纹的复核资料包，`POST .../review` 提交绑定版本和指纹的独立复核，`POST .../activate` 签发许可。
- `GET .../reviews`（或 `.../review-history`）查询独立复核历史，支持 `decision`、`reviewerName`、`reviewedAtFrom`、`reviewedAtTo` 过滤；`GET .../export` 导出确定性档案快照并返回 SHA-256。
- `GET /api/v1/commissioning-cases` 支持 `state`、`zoneCode`、`ownerName`、`updatedAtFrom`、`updatedAtTo`、`page` 和 `pageSize` 组合查询。
- `GET /api/v1/permits/{permitCode}` 查询原始许可，`GET .../validation` 动态验证单个许可；`POST /api/v1/permits/validation` 以 `{"permitCodes":[...]}` 批量核验最多 100 个许可并返回逐项结果与汇总。

除建档和查询外，变更请求使用 `X-Expected-Version`（或 `expectedVersion` 查询参数）进行乐观锁校验，并可通过 `Idempotency-Key` 安全重试。复核请求体还必须携带资料包返回的 `reviewedVersion` 和 `packageFingerprint`。
