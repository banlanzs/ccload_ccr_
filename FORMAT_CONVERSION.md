# 格式转换配置指南

## 功能说明

ccLoad 支持三种 API 格式的互相转换：
- **OpenAI** 格式
- **Anthropic** (Claude) 格式
- **Gemini** 格式

支持所有 6 种转换路径：
1. OpenAI → Anthropic
2. OpenAI → Gemini
3. Anthropic → OpenAI
4. Anthropic → Gemini
5. Gemini → OpenAI
6. Gemini → Anthropic

## 配置方式

### 方式 1：自动检测（推荐）

系统会自动检测请求格式并根据渠道类型转换。

**渠道配置示例**：
```json
{
  "id": 1,
  "name": "Claude API",
  "channel_type": "anthropic",
  "url": "https://api.anthropic.com",
  "enable_conversion": true
}
```

**工作流程**：
1. 自动检测请求格式（如 OpenAI）
2. 根据 `channel_type` 推断目标格式（如 Anthropic）
3. 自动转换 OpenAI → Anthropic

**日志输出**：
```
[INFO] Channel Claude API (ID=1): Auto-detected source format: openai
[INFO] Channel Claude API (ID=1): Inferred target format from channel_type=anthropic: anthropic
[INFO] Channel Claude API (ID=1): Converting openai -> anthropic
[INFO] Channel Claude API (ID=1): Conversion succeeded, payload size: 150 -> 180 bytes
```

### 方式 2：显式配置（精确控制）

明确指定源格式和目标格式，不依赖自动检测。

**渠道配置示例**：
```json
{
  "id": 2,
  "name": "Gemini Proxy",
  "channel_type": "gemini",
  "url": "https://generativelanguage.googleapis.com",
  "enable_conversion": true,
  "conversion_source_format": "openai",
  "conversion_target_format": "gemini"
}
```

**优势**：
- 避免自动检测错误
- 明确转换路径
- 便于调试和维护

**日志输出**：
```
[INFO] Channel Gemini Proxy (ID=2): Using configured source format: openai
[INFO] Channel Gemini Proxy (ID=2): Using configured target format: gemini
[INFO] Channel Gemini Proxy (ID=2): Converting openai -> gemini
[INFO] Channel Gemini Proxy (ID=2): Conversion succeeded, payload size: 150 -> 200 bytes
```

## 配置字段说明

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `enable_conversion` | bool | 是 | 是否启用格式转换 |
| `conversion_source_format` | string | 否 | 源格式：`openai`/`anthropic`/`gemini`，空表示自动检测 |
| `conversion_target_format` | string | 否 | 目标格式：`openai`/`anthropic`/`gemini`，空表示根据 `channel_type` 推断 |

## 使用场景

### 场景 1：OpenAI SDK 调用 Claude API

**需求**：使用 OpenAI SDK 调用 Claude API

**配置**：
```json
{
  "name": "Claude via OpenAI SDK",
  "channel_type": "anthropic",
  "url": "https://api.anthropic.com",
  "enable_conversion": true,
  "conversion_source_format": "openai",
  "conversion_target_format": "anthropic"
}
```

**客户端代码**（无需修改）：
```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:8088/v1")
response = client.chat.completions.create(
    model="claude-3-opus",
    messages=[{"role": "user", "content": "Hello"}]
)
```

### 场景 2：Claude SDK 调用 Gemini API

**需求**：使用 Anthropic SDK 调用 Gemini API

**配置**：
```json
{
  "name": "Gemini via Claude SDK",
  "channel_type": "gemini",
  "url": "https://generativelanguage.googleapis.com",
  "enable_conversion": true,
  "conversion_source_format": "anthropic",
  "conversion_target_format": "gemini"
}
```

**客户端代码**：
```python
from anthropic import Anthropic

client = Anthropic(base_url="http://localhost:8088")
response = client.messages.create(
    model="gemini-pro",
    messages=[{"role": "user", "content": "Hello"}]
)
```

### 场景 3：统一 OpenAI 接口

**需求**：所有渠道统一使用 OpenAI 格式

**配置多个渠道**：
```json
[
  {
    "name": "Claude",
    "channel_type": "anthropic",
    "enable_conversion": true,
    "conversion_target_format": "anthropic"
  },
  {
    "name": "Gemini",
    "channel_type": "gemini",
    "enable_conversion": true,
    "conversion_target_format": "gemini"
  },
  {
    "name": "OpenAI",
    "channel_type": "openai",
    "enable_conversion": false
  }
]
```

**客户端代码**（统一使用 OpenAI SDK）：
```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:8088/v1")

# 自动路由到不同渠道并转换格式
response = client.chat.completions.create(
    model="claude-3-opus",  # 自动转换为 Anthropic 格式
    messages=[{"role": "user", "content": "Hello"}]
)
```

## 日志级别

转换过程会输出详细日志，便于调试：

| 级别 | 说明 | 示例 |
|------|------|------|
| `[INFO]` | 正常转换流程 | `Auto-detected source format: openai` |
| `[INFO]` | 转换成功 | `Conversion succeeded, payload size: 150 -> 180 bytes` |
| `[WARN]` | 转换失败（回退） | `Format conversion failed: ..., using original payload` |
| `[WARN]` | 无法检测格式 | `Unable to detect source format, skipping conversion` |

## 故障排查

### 问题 1：转换未生效

**症状**：请求失败，日志无转换信息

**检查**：
1. 确认 `enable_conversion: true`
2. 检查日志是否有 `[INFO] Converting ...` 输出
3. 验证源格式和目标格式配置

### 问题 2：转换失败

**症状**：日志显示 `[WARN] Format conversion failed`

**原因**：
- 请求体格式不符合规范
- 包含不支持的字段
- JSON 解析错误

**解决**：
1. 查看完整错误信息
2. 验证请求体 JSON 格式
3. 使用显式配置避免自动检测错误

### 问题 3：无法检测源格式

**症状**：日志显示 `Unable to detect source format`

**原因**：
- 请求体缺少特征字段
- 自定义格式无法识别

**解决**：
使用显式配置 `conversion_source_format`

## 性能影响

- **自动检测**：~0.1ms（JSON 字段检查）
- **格式转换**：~1-2ms（JSON 解析 + 转换 + 序列化）
- **总体影响**：< 5% 延迟增加

## 与 Legacy CCR 的关系

新格式转换系统与 legacy CCR transformer 完全共存：

| 特性 | Legacy CCR | 新格式转换系统 |
|------|-----------|---------------|
| 支持格式 | OpenAI ↔ Anthropic | OpenAI ↔ Anthropic ↔ Gemini |
| 配置方式 | `enable_ccr` + `ccr_transformer` | `enable_conversion` + 可选显式配置 |
| 优先级 | 低 | 高（先执行） |
| 推荐使用 | ❌ 已废弃 | ✅ 推荐 |

**迁移建议**：
```json
// 旧配置（Legacy）
{
  "enable_ccr": true,
  "ccr_transformer": "openai_to_claude"
}

// 新配置（推荐）
{
  "enable_conversion": true,
  "conversion_source_format": "openai",
  "conversion_target_format": "anthropic"
}
```

## API 管理界面配置

通过 Web 界面配置格式转换：

1. 访问 `http://localhost:8088/web/channels.html`
2. 编辑渠道配置
3. 勾选 "启用格式转换"
4. （可选）设置源格式和目标格式
5. 保存配置

配置立即生效，无需重启服务。

## 测试验证

### 测试所有转换路径

```bash
cd ccLoad
go test -tags go_json ./internal/ccr/... -run TestAllConversionPaths -v
```

**预期输出**：
```
PASS: TestAllConversionPaths/OpenAI_->_Anthropic
PASS: TestAllConversionPaths/OpenAI_->_Gemini
PASS: TestAllConversionPaths/Anthropic_->_OpenAI
PASS: TestAllConversionPaths/Anthropic_->_Gemini
PASS: TestAllConversionPaths/Gemini_->_OpenAI
PASS: TestAllConversionPaths/Gemini_->_Anthropic
```

### 实际请求测试

```bash
# 测试 OpenAI -> Anthropic 转换
curl -X POST http://localhost:8088/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# 查看日志确认转换
# 应该看到：[INFO] Converting openai -> anthropic
```

## 常见问题

**Q: 是否支持流式响应转换？**
A: 是的，格式转换对流式和非流式请求都生效。

**Q: 转换会影响性能吗？**
A: 影响很小（< 5% 延迟），JSON 解析和转换都是内存操作。

**Q: 可以同时启用多个转换吗？**
A: 每个渠道独立配置，可以为不同渠道配置不同的转换路径。

**Q: 转换失败会怎样？**
A: 自动回退到原始请求体，记录 WARN 日志，不影响请求继续执行。

**Q: 如何禁用转换？**
A: 设置 `enable_conversion: false` 或删除该字段。
