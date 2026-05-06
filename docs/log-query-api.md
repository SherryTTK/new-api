## 日志查询接口

通过 API 令牌查询消费日志，支持按时间范围、模型名称、令牌名称筛选。

### 请求

```
GET /api/log/token/query
```

**鉴权方式：** Bearer Token

```
Authorization: Bearer sk-xxx
```

### 请求参数（Query）

| 参数         | 类型     | 必选  | 说明                            |
| ---------- | ------ | --- | ----------------------------- |
| start_time | string | 是   | 开始时间，格式 `2006-01-02 15:04:05` |
| end_time   | string | 是   | 结束时间，格式 `2006-01-02 15:04:05` |
| model_name | string | 否   | 模型名称，精确匹配                     |
| token_name | string | 否   | 令牌名称，精确匹配                     |

### 响应

```json
{
  "success": true,
  "message": "",
  "data": [
    {
      "request_id": "xxx",
      "created_at": "2026-04-01 00:00:00",
      "username": "user1",
      "token_name": "my-token",
      "token_id": 1,
      "model_name": "gpt-4o",
      "prompt_tokens": 100,
      "completion_tokens": 50,
      "cache_creation_tokens": 0,
      "cache_read_tokens": 0,
      "quota": 500000,
      "consume": 1.0,
      "use_time": 3,
      "is_stream": false,
      "group": "default",
      "channel_id": 1,
      "content": ""
    }
  ]
}
```

### 字段说明

| 字段                    | 类型     | 说明                            |
| --------------------- | ------ | ----------------------------- |
| request_id            | string | 请求 ID                         |
| created_at            | string | 创建时间，格式 `YYYY-MM-DD HH:mm:ss` |
| username              | string | 用户名                           |
| token_name            | string | 令牌名称                          |
| token_id              | int    | 令牌 ID                         |
| model_name            | string | 模型名称                          |
| prompt_tokens         | int    | 输入 token 数                    |
| completion_tokens     | int    | 输出 token 数                    |
| cache_creation_tokens | int    | 缓存创建 token 数                  |
| cache_read_tokens     | int    | 缓存读取 token 数                  |
| quota                 | int    | 消耗额度（内部单位）                    |
| consume               | float  | 实际消耗金额（USD）                   |
| use_time              | int    | 耗时（秒）                         |
| is_stream             | bool   | 是否流式请求                        |
| group                 | string | 分组                            |
| channel_id            | int    | 渠道 ID                         |
| content               | string | 日志内容                          |

### 错误响应示例

```json
{
  "success": false,
  "message": "start_time 和 end_time 为必填参数"
}
```

### 请求示例

```bash
curl -H "Authorization: Bearer sk-xxx" \
  "https://your-domain/api/log/token/query?start_time=2026-04-01%2000:00:00&end_time=2026-04-30%2023:59:59&model_name=gpt-4o"
```
