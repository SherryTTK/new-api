# 按令牌固定回复

在默认主题的「API 密钥」新增或编辑表单中开启「固定回复」，设置最短/最长等待时间（毫秒）及回复内容。配置属于当前令牌，不需要重新填写密钥。

开启后，支持的文本生成请求通过正常鉴权、模型权限、请求校验与限流后，在本地生成响应，不选择渠道、不请求上游、不执行工具。配置关闭后恢复正常转发。非文本生成接口保持原有行为。

## 接口范围

- `/v1/chat/completions`
- `/v1/completions`
- `/v1/responses`
- `/v1/messages`
- `/v1beta/models/{model}:generateContent`、`:streamGenerateContent`
- `/v1/models/{model}:generateContent`、`:streamGenerateContent`

以上接口支持非流式及其对应的流式协议。流式请求在等待后一次发送完整事件序列，不模拟逐字输出。OpenAI 流式 usage 遵循 `stream_options.include_usage`；Claude、Responses 和 Gemini 使用各自的原生用量字段。Gemini `streamGenerateContent` 支持 `alt=sse`，否则返回 JSON 数组。

Embeddings、Rerank、Moderation、Responses Compact、Realtime、图像、音频和异步任务不属于此功能的拦截范围。

## 计费与限制

- 输入按实际请求内容进行本地 Token 统计，输出按固定文本统计；即使全局预估 Token 开关关闭，也会统计。
- OpenAI 文本模型使用现有 tokenizer，其他模型使用现有估算规则。多模态附件使用本地保守估算，不下载远程媒体；结果不等同于供应商实际 usage。
- 复用模型价格、分组倍率、按次计费和表达式计费规则，并沿用钱包/订阅计费偏好。固定回复不会收取未执行工具的调用费用。
- 在等待前预扣完整回复所需额度，成功写出后结算。等待期间取消或响应写入失败时退还预扣额度；退款沿用现有异步机制。
- 令牌失效、额度不足、模型无权限、缺少必要定价时仍会报错。命中固定回复后不回退上游。
- 消费日志的渠道 ID 为 0，`other.fixed_response` 为 `true`，并记录输入/输出 Token 与额度。不会增加真实渠道的消耗。
- 使用 `auto` 分组时，选择用户可用自动分组列表的第一个分组计费，不依赖该分组是否存在可用渠道。
- 每个请求返回一份固定文本。生成参数不改变内容；不执行工具、不保证满足客户端 JSON Schema，也不按 `max_tokens` 截断固定文本。
- 等待范围必须满足 `0 <= min_delay_ms <= max_delay_ms <= 300000`，每次请求均匀随机取值。内容必须为非空 UTF-8 文本，最多 64 KiB。
- 等待范围是服务端模拟延迟，不包含请求处理、网络传输及客户端开销；部署时仍需保证客户端和代理超时大于等待时间。

## API 配置

令牌创建/更新接口接受以下字段（更新时仍需提交原有令牌编辑字段）：

```json
{
  "fixed_response": {
    "enabled": true,
    "min_delay_ms": 1000,
    "max_delay_ms": 3000,
    "content": "这是固定回复。"
  }
}
```

更新时省略 `fixed_response` 或传 `null` 会保留原配置，兼容旧客户端。关闭时明确提交 `enabled: false`。配置存储为数据库 TEXT 中的 JSON，支持 SQLite、MySQL 和 PostgreSQL；Redis 令牌缓存使用相同 JSON 内容。
