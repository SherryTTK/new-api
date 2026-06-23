## 沙盒组织与 API Key 接口文档

本文档定义 5 个沙盒相关接口，并附带联调测试 `curl` 示例。

### 设计范围

- API Key 在系统内部对应 `token`
- 所有沙盒 Key 都绑定到专用用户 `sandbox`
- 组织与 Key 为一对多关系
- 同一个 `organization_id` 可多次创建新 Key
- 创建新 Key 时不影响旧 Key，旧 Key 保持原状

### 联调环境

- Base URL：`https://hk.apitoken.ai`
- 固定请求头：`X-Sandbox-Secret: gkmBHwR7EXHtzukrEjtf`
- 当前实现中，这 5 个沙盒接口不受应用内请求限流影响

### 鉴权约定

#### 1. 内部接口：专用 Secret

用于接口 `1` 和 `3`。

```http
X-Sandbox-Secret: gkmBHwR7EXHtzukrEjtf
```

说明：

- 不复用 `Authorization`
- 仅用于服务间调用

#### 2. 自助接口：当前 Key 自己调用自己

用于接口 `2`、`4` 和 `5`。

```http
Authorization: Bearer sk-xxxxxxxx
```

说明：

- 使用当前目标 Key 自己鉴权
- 后端实现应使用 `TokenAuthReadOnly`
- 因此即使 Key 已过期、耗尽或被禁用，仍可调用接口 `2` 完成恢复
- 但若所属用户 `sandbox` 被禁用，则接口仍不可用

### 数据表设计

#### 1. `sandbox_organizations`

用于记录沙盒组织主数据。

| 字段              | 类型            | 必填  | 约束   | 说明          |
| --------------- | ------------- | --- | ---- | ----------- |
| id              | int           | 是   | 主键   | 自增 ID       |
| organization_id | varchar(64)   | 是   | 唯一索引 | 业务组织 ID     |
| created_time    | bigint        | 是   | 索引   | 创建时间，Unix 秒 |
| updated_time    | bigint        | 是   |      | 更新时间，Unix 秒 |
| deleted_at      | datetime/null | 否   | 索引   | 软删除         |

说明：

- `organization_id` 全局唯一
- 接口 `1` 建议按幂等方式设计：重复创建同一组织时，直接返回已存在记录

#### 2. `sandbox_org_tokens`

用于记录组织与 Key 的历史关联。

| 字段                      | 类型            | 必填  | 约束   | 说明                            |
| ----------------------- | ------------- | --- | ---- | ----------------------------- |
| id                      | int           | 是   | 主键   | 自增 ID                         |
| sandbox_organization_id | int           | 是   | 索引   | 关联 `sandbox_organizations.id` |
| token_id                | int           | 是   | 唯一索引 | 关联系统 `tokens.id`              |
| created_time            | bigint        | 是   | 索引   | 关联创建时间，Unix 秒                 |
| deleted_at              | datetime/null | 否   | 索引   | 软删除                           |

说明：

- 一条记录代表“某次为某个组织生成了一把 Key”
- 同一个组织可对应多条记录
- 新建 Key 时仅新增一条关联记录，不处理旧记录

### 固定配置项、默认值与金额单位

接口 `3` 创建 Key 时，以下字段由后端固定写死，不从请求传入：

| 字段                     | 说明                      |
| ---------------------- | ----------------------- |
| `user_id`              | 固定为用户 `sandbox` 的 ID    |
| `unlimited_quota`      | 固定为 `false`             |
| `model_limits_enabled` | 固定值                     |
| `model_limits`         | 固定值                     |
| `allow_ips`            | 固定值                     |
| `cross_group_retry`    | 固定值                     |
| `token.name`           | 固定为 `组织id + 当前时间(精确到秒)` |

接口 `3` 允许调用方显式传入以下初始配置：

| 字段                  | 说明                              |
| ------------------- | ------------------------------- |
| `remain_amount_usd` | 新 Key 的初始额度，单位 USD              |
| `expired_time`      | 新 Key 的初始有效期，Unix 秒；`-1` 表示永不过期 |
| `group`             | 新 Key 的分组；仅支持 `sandbox` 和 `sandbox-China` |

接口 `3` 的默认值：

| 字段                  | 默认值   | 说明            |
| ------------------- | ----- | ------------- |
| `remain_amount_usd` | `5.0` | 请求未传时，默认 5 美元 |
| `expired_time`      | `-1`  | 请求未传时，默认永久有效  |
| `group`             | `sandbox` | 请求未传或传空字符串时，默认 `sandbox` |

金额单位约定：

- 对外接口中所有额度相关字段统一使用 USD
- 系统内部仍使用 `quota` 存储和计算
- 后端按运行时 `QuotaPerUnit` 配置进行换算
- 换算公式：
  - `internal_quota = amount_usd * QuotaPerUnit`
  - `amount_usd = internal_quota / QuotaPerUnit`
- 当前代码默认 `QuotaPerUnit = 500000`，即默认 `1 USD = 500000 quota`

建议增加以下运行前置检查：

- `sandbox` 用户必须存在
- `sandbox` 用户必须为启用状态
- `sandbox` 用户必须具备目标分组的可用权限
- `sandbox` 用户额度需由后台定期充值保障

### 通用响应格式

成功：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

失败：

```json
{
  "success": false,
  "message": "error message"
}
```

---

## 1. 沙盒组织创建接口

记录一个新的沙盒组织。

### 请求

```http
POST /api/sandbox/organizations
```

### 鉴权方式

```http
X-Sandbox-Secret: gkmBHwR7EXHtzukrEjtf
```

### 请求体

```json
{
  "organization_id": "org-123456"
}
```

### 请求参数

| 字段              | 类型     | 必填  | 说明    |
| --------------- | ------ | --- | ----- |
| organization_id | string | 是   | 组织 ID |

### 处理规则

- 若组织不存在，则创建新记录
- 若组织已存在，则直接返回已存在记录
- 本接口只记录组织，不创建 Key

### 成功响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "organization_id": "org-123456",
    "created": true,
    "created_time": 1780675200,
    "updated_time": 1780675200
  }
}
```

### 重复创建响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "organization_id": "org-123456",
    "created": false,
    "created_time": 1780675200,
    "updated_time": 1780675200
  }
}
```

### 错误场景

- `X-Sandbox-Secret` 错误或缺失
- `organization_id` 为空

### 请求示例

```bash
curl -X POST "https://hk.apitoken.ai/api/sandbox/organizations" \
  -H "Content-Type: application/json" \
  -H "X-Sandbox-Secret: gkmBHwR7EXHtzukrEjtf" \
  -d '{
    "organization_id": "org-123456"
  }'
```

---

## 2. API Key 配置修改接口

当前 Key 自己调用自己，修改自己的额度和有效期，并自动恢复可用状态。

### 请求

```http
PUT /api/sandbox/token/self
```

### 鉴权方式

```http
Authorization: Bearer sk-xxxxxxxx
```

### 请求体

```json
{
  "remain_amount_usd": 5.0,
  "expired_time": 1783267200
}
```

### 请求参数

| 字段                | 类型     | 必填  | 说明                          |
| ----------------- | ------ | --- | --------------------------- |
| remain_amount_usd | number | 是   | 新额度，单位 USD，直接覆盖当前 Key 的剩余额度 |
| expired_time      | int64  | 是   | 新过期时间，Unix 秒；`-1` 表示永不过期    |

### 处理规则

- 只修改当前鉴权 Key 对应的 `token`
- 修改成功后，统一将当前 Key 的 `status` 恢复为 `enabled`
- 不修改 `group`
- 不修改组织关联关系
- 不处理同组织下的其他 Key

### 建议校验规则

- `remain_amount_usd >= 0`
- `expired_time == -1` 或 `expired_time > 当前时间`

### 成功响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "token_id": 101,
    "status": 1,
    "remain_amount_usd": 5.0,
    "expired_time": 1783267200,
    "auto_recovered": true
  }
}
```

### 错误场景

- `Authorization` 缺失或格式错误
- 当前 Key 不存在
- `remain_amount_usd` 非法
- `expired_time` 非法
- `sandbox` 用户被禁用

### 请求示例

```bash
curl -X PUT "https://hk.apitoken.ai/api/sandbox/token/self" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -d '{
    "remain_amount_usd": 5.0,
    "expired_time": 1783267200
  }'
```

---

## 3. API Key 创建接口

为指定组织新增一把新的沙盒 Key，并建立新的组织-Key 关联记录。

### 请求

```http
POST /api/sandbox/organizations/:organization_id/tokens
```

### 鉴权方式

```http
X-Sandbox-Secret: gkmBHwR7EXHtzukrEjtf
```

### 请求体

```json
{
  "remain_amount_usd": 5.0,
  "expired_time": -1,
  "group": "sandbox"
}
```

### 路径参数

| 参数              | 类型     | 必填  | 说明      |
| --------------- | ------ | --- | ------- |
| organization_id | string | 是   | 目标组织 ID |

### 请求参数

| 字段                | 类型     | 必填  | 说明                             |
| ----------------- | ------ | --- | ------------------------------ |
| remain_amount_usd | number | 否   | 新 Key 的初始额度，单位 USD；未传时默认 `5.0` |
| expired_time      | int64  | 否   | 新 Key 的初始有效期，Unix 秒；未传时默认 `-1` |
| group             | string | 否   | 新 Key 的分组；仅支持 `sandbox`、`sandbox-China`；未传或传空时默认 `sandbox` |

说明：

- 当两个字段都使用默认值时，请求体可为空对象 `{}`

### 处理规则

- 要求 `organization_id` 已存在于 `sandbox_organizations`
- 每次调用都新建一把 Key
- 每次调用都新增一条 `sandbox_org_tokens` 记录
- 不禁用旧 Key
- 不删除旧 Key
- 新 Key 绑定到专用用户 `sandbox`
- `remain_amount_usd` 未传时默认按 `5.0 USD` 创建
- `expired_time` 未传时默认按永久有效创建
- `group` 仅支持 `sandbox` 和 `sandbox-China`
- `group` 未传或传空字符串时默认按 `sandbox` 创建
- `token.name` 固定为 `organization_id + 当前时间(精确到秒)`，建议格式为 `{organization_id}-{yyyyMMddHHmmss}`
- `user_id`、`unlimited_quota`、`model_limits_enabled`、`model_limits`、`allow_ips`、`cross_group_retry` 等配置仍由后端固定
- 后端收到 USD 金额后需换算成系统内部 `quota` 再写入 `token.remain_quota`

### 建议命名规则

新 Key 的 `token.name` 固定为组织 ID 加当前时间，便于日志排查并区分多次发放，例如：

```text
org-123456-20260608153045
```

若名称长度超过系统限制，建议保留前缀并追加短哈希，但整体规则仍保持“组织 ID + 秒级时间”可识别。

### 建议校验规则

- `remain_amount_usd >= 0`
- `expired_time == -1` 或 `expired_time > 当前时间`
- `group` 仅允许 `sandbox` 或 `sandbox-China`

### 成功响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "organization_id": "org-123456",
    "sandbox_organization_id": 1,
    "relation_id": 12,
    "token_id": 101,
    "name": "org-123456-20260608153045",
    "key": "sk-abc123xyz456",
    "group": "sandbox",
    "remain_amount_usd": 5.0,
    "expired_time": -1,
    "created_time": 1780675200
  }
}
```

### 错误场景

- `X-Sandbox-Secret` 错误或缺失
- `organization_id` 不存在
- `remain_amount_usd` 非法
- `expired_time` 非法
- `group` 非法
- `sandbox` 用户不存在
- `sandbox` 用户被禁用
- `sandbox` 用户不具备目标分组权限
- `sandbox` 用户 token 数量已达到系统限制
- `sandbox` 用户余额不足以支撑后续使用

### 请求示例

```bash
curl -X POST "https://hk.apitoken.ai/api/sandbox/organizations/org-123456/tokens" \
  -H "Content-Type: application/json" \
  -H "X-Sandbox-Secret: gkmBHwR7EXHtzukrEjtf" \
  -d '{
    "remain_amount_usd": 8.5,
    "expired_time": 1783267200,
    "group": "sandbox-China"
  }'
```

---

## 4. API Key 删除接口

当前 Key 自己调用自己，删除自己的 Key。

### 请求

```http
DELETE /api/sandbox/token/self
```

### 鉴权方式

```http
Authorization: Bearer sk-xxxxxxxx
```

### 处理规则

- 只删除当前鉴权 Key 对应的 `token`
- 删除方式建议与系统现有 `token` 保持一致，采用软删除
- 不删除 `sandbox_organizations` 记录
- 不删除 `sandbox_org_tokens` 历史关联记录
- 删除后该 Key 立即失效，不能再用于任何接口调用

### 成功响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "token_id": 101,
    "deleted": true
  }
}
```

### 错误场景

- `Authorization` 缺失或格式错误
- 当前 Key 不存在
- `sandbox` 用户被禁用

### 请求示例

```bash
curl -X DELETE "https://hk.apitoken.ai/api/sandbox/token/self" \
  -H "Authorization: Bearer sk-xxxxxxxx"
```

---

## 5. API Key 日志查询接口

当前 Key 自己查询自己的消费日志，并可额外返回另一个统计时间段的总用量。

### 请求

```http
GET /api/sandbox/log/self
```

### 鉴权方式

```http
Authorization: Bearer sk-xxxxxxxx
```

### Query 参数

| 参数                      | 类型    | 必填  | 说明                    |
| ----------------------- | ----- | --- | --------------------- |
| log_start_timestamp     | int64 | 否   | 日志明细开始时间，Unix 秒       |
| log_end_timestamp       | int64 | 否   | 日志明细结束时间，Unix 秒       |
| summary_start_timestamp | int64 | 否   | 总用量统计开始时间，Unix 秒      |
| summary_end_timestamp   | int64 | 否   | 总用量统计结束时间，Unix 秒      |
| p                       | int   | 否   | 页码，默认 `1`             |
| size                    | int   | 否   | 每页条数，默认 `10`，最大 `100` |

### 处理规则

- 仅返回当前鉴权 Key 对应 `token_id` 的消费日志
- 默认按 `id desc` 返回
- `log_start_timestamp` 和 `log_end_timestamp` 用于控制日志明细范围
- `summary_start_timestamp` 和 `summary_end_timestamp` 用于统计总用量范围
- 两个时间段彼此独立，互不影响
- 当 `log_start_timestamp` 和 `log_end_timestamp` 都为空时，返回该 Key 的全部日志，但必须分页
- 当仅传一个日志时间参数时，按单边时间范围过滤
- `summary_start_timestamp` 和 `summary_end_timestamp` 必须成对出现
- 仅查询消费日志，不返回其他类型日志
- 统计总用量时，建议返回以下聚合结果：
  - 总请求数
  - 总输入 token 数
  - 总输出 token 数
  - 总额度消耗（USD）

### 成功响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "page": 1,
    "page_size": 10,
    "total": 2,
    "usage_summary": {
      "start_timestamp": 1780675200,
      "end_timestamp": 1780761600,
      "total_requests": 2,
      "total_prompt_tokens": 220,
      "total_completion_tokens": 110,
      "total_amount_usd": 2.0
    },
    "items": [
      {
        "request_id": "req_001",
        "created_at": "2026-06-06 10:00:00",
        "username": "sandbox",
        "token_name": "org-123456-20260608153045",
        "token_id": 101,
        "model_name": "gpt-4o",
        "prompt_tokens": 100,
        "completion_tokens": 50,
        "cache_creation_tokens": 0,
        "cache_read_tokens": 0,
        "amount_usd": 1.0,
        "use_time": 3,
        "is_stream": false,
        "group": "sandbox",
        "content": ""
      }
    ]
  }
}
```

### 字段说明

| 字段                    | 类型     | 说明                            |
| --------------------- | ------ | ----------------------------- |
| request_id            | string | 请求 ID                         |
| created_at            | string | 日志时间，格式 `YYYY-MM-DD HH:mm:ss` |
| username              | string | 用户名，固定为 `sandbox`             |
| token_name            | string | Key 名称                        |
| token_id              | int    | Key 对应的 token ID              |
| model_name            | string | 模型名称                          |
| prompt_tokens         | int    | 输入 token 数                    |
| completion_tokens     | int    | 输出 token 数                    |
| cache_creation_tokens | int    | 缓存创建 token 数                  |
| cache_read_tokens     | int    | 缓存读取 token 数                  |
| amount_usd            | float  | 本条日志消耗金额，单位 USD               |
| use_time              | int    | 耗时，秒                          |
| is_stream             | bool   | 是否流式                          |
| group                 | string | 分组                            |
| content               | string | 日志内容                          |

`usage_summary` 字段说明：

| 字段                      | 类型    | 说明            |
| ----------------------- | ----- | ------------- |
| start_timestamp         | int64 | 统计开始时间，Unix 秒 |
| end_timestamp           | int64 | 统计结束时间，Unix 秒 |
| total_requests          | int   | 总请求数          |
| total_prompt_tokens     | int   | 总输入 token 数   |
| total_completion_tokens | int   | 总输出 token 数   |
| total_amount_usd        | float | 总消耗金额，单位 USD  |

### 错误场景

- `Authorization` 缺失或格式错误
- 当前 Key 不存在
- 日志时间参数格式错误
- 统计时间参数缺失一半或格式错误
- `sandbox` 用户被禁用

### 请求示例

```bash
curl "https://hk.apitoken.ai/api/sandbox/log/self?log_start_timestamp=1780675200&log_end_timestamp=1780761600&summary_start_timestamp=1780588800&summary_end_timestamp=1781193600&p=1&size=10" \
  -H "Authorization: Bearer sk-xxxxxxxx"
```

---

## 测试 Curl 汇总

建议先准备环境变量：

```bash
BASE_URL="https://hk.apitoken.ai"
SECRET="gkmBHwR7EXHtzukrEjtf"
ORG_ID="org-123456"
SANDBOX_KEY="sk-替换成实际返回的key"
```

### 1. 创建沙盒组织

```bash
curl -X POST "$BASE_URL/api/sandbox/organizations" \
  -H "Content-Type: application/json" \
  -H "X-Sandbox-Secret: $SECRET" \
  -d '{
    "organization_id": "'"$ORG_ID"'"
  }'
```

### 2. 为组织新增 Key，使用默认额度与默认有效期

默认行为：

- `remain_amount_usd = 5.0`
- `expired_time = -1`
- `group = sandbox`

```bash
curl -X POST "$BASE_URL/api/sandbox/organizations/$ORG_ID/tokens" \
  -H "Content-Type: application/json" \
  -H "X-Sandbox-Secret: $SECRET" \
  -d '{}'
```

### 3. 为组织新增 Key，指定初始额度与有效期

```bash
curl -X POST "$BASE_URL/api/sandbox/organizations/$ORG_ID/tokens" \
  -H "Content-Type: application/json" \
  -H "X-Sandbox-Secret: $SECRET" \
  -d '{
    "remain_amount_usd": 8.5,
    "expired_time": 1783267200
  }'
```

### 3.1 为组织新增 Key，并显式指定分组

```bash
curl -X POST "$BASE_URL/api/sandbox/organizations/$ORG_ID/tokens" \
  -H "Content-Type: application/json" \
  -H "X-Sandbox-Secret: $SECRET" \
  -d '{
    "remain_amount_usd": 8.5,
    "expired_time": 1783267200,
    "group": "sandbox-China"
  }'
```

### 4. 当前 Key 自助修改额度与有效期

```bash
curl -X PUT "$BASE_URL/api/sandbox/token/self" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $SANDBOX_KEY" \
  -d '{
    "remain_amount_usd": 3.25,
    "expired_time": 1783267200
  }'
```

### 5. 当前 Key 自助删除

```bash
curl -X DELETE "$BASE_URL/api/sandbox/token/self" \
  -H "Authorization: Bearer $SANDBOX_KEY"
```

### 6. 查询当前 Key 的全部日志

```bash
curl "$BASE_URL/api/sandbox/log/self" \
  -H "Authorization: Bearer $SANDBOX_KEY"
```

### 7. 查询当前 Key 的日志明细与汇总时间段

```bash
curl "$BASE_URL/api/sandbox/log/self?log_start_timestamp=1780675200&log_end_timestamp=1780761600&summary_start_timestamp=1780588800&summary_end_timestamp=1781193600&p=1&size=10" \
  -H "Authorization: Bearer $SANDBOX_KEY"
```

---

## 当前实现说明

### 当前路由

| 接口       | 方法       | 路径                                                   | 鉴权                             |
| -------- | -------- | ---------------------------------------------------- | ------------------------------ |
| 沙盒组织创建   | `POST`   | `/api/sandbox/organizations`                         | `X-Sandbox-Secret`             |
| Key 配置修改 | `PUT`    | `/api/sandbox/token/self`                            | `Authorization: Bearer sk-...` |
| Key 创建   | `POST`   | `/api/sandbox/organizations/:organization_id/tokens` | `X-Sandbox-Secret`             |
| Key 删除   | `DELETE` | `/api/sandbox/token/self`                            | `Authorization: Bearer sk-...` |
| Key 日志查询 | `GET`    | `/api/sandbox/log/self`                              | `Authorization: Bearer sk-...` |

### 关键业务语义

- 接口 `1` 只建组织，不建 Key
- 接口 `3` 每次调用都新增一把 Key
- 接口 `3` 不处理旧 Key
- 接口 `2` 修改后自动恢复 Key 状态
- 接口 `4` 删除当前 Key 自己
- 接口 `5` 只查当前 Key 自己的消费日志
- 接口 `5` 支持独立的明细时间段和总用量统计时间段
