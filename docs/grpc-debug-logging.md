# gRPC 调试日志使用说明

## 功能介绍

在 DEBUG 日志级别下，gRPC 客户端会自动记录所有 RPC 调用的详细信息，包括：
- RPC 方法名
- 请求参数（JSON 格式）
- 响应数据（JSON 格式）
- 调用耗时（毫秒）
- 错误信息（如果有）

## 配置方式

在配置文件中设置日志级别为 `debug`：

```yaml
Log:
  Level: debug    # 启用 DEBUG 日志
  Output: ""      # 留空输出到控制台，或指定文件路径
  Access: "none"
```

## 日志输出示例

### 成功的 gRPC 调用

```json
{
  "level": "debug",
  "msg": "gRPC request sent",
  "method": "/ppanel.nodecontrol.v1.NodeControlService/GetConfig",
  "request": "{\"server_id\":\"1\", \"protocols\":[\"trojan\"], \"known_revision\":\"abc123\"}",
  "time": "2026-02-13T10:30:45Z"
}

{
  "level": "debug",
  "msg": "gRPC call completed",
  "method": "/ppanel.nodecontrol.v1.NodeControlService/GetConfig",
  "duration_ms": 45,
  "response": "{\"changed\":true, \"config\":{...}}",
  "time": "2026-02-13T10:30:45Z"
}
```

### 失败的 gRPC 调用

```json
{
  "level": "debug",
  "msg": "gRPC request sent",
  "method": "/ppanel.nodecontrol.v1.NodeControlService/GetUserList",
  "request": "{\"server_id\":\"1\", \"protocol\":\"trojan\", \"known_revision\":\"\"}",
  "time": "2026-02-13T10:31:20Z"
}

{
  "level": "debug",
  "msg": "gRPC call failed",
  "method": "/ppanel.nodecontrol.v1.NodeControlService/GetUserList",
  "duration_ms": 1523,
  "error": "rpc error: code = Unauthenticated desc = invalid credentials",
  "time": "2026-02-13T10:31:21Z"
}
```

## 使用场景

1. **调试 API 集成问题** - 查看实际发送和接收的数据
2. **性能分析** - 通过 `duration_ms` 字段分析 RPC 调用耗时
3. **错误排查** - 查看完整的错误信息和请求上下文

## 注意事项

1. **生产环境建议** - 不要在生产环境使用 DEBUG 级别，会产生大量日志
2. **敏感信息** - DEBUG 日志会记录完整的请求和响应，可能包含敏感信息（如 API 密钥、用户数据），请妥善保管日志文件
3. **性能影响** - JSON 序列化会有轻微的性能开销，但仅在 DEBUG 级别启用时才会执行

## 实现细节

gRPC 调试日志通过 Unary Interceptor 实现，具体代码在：
- `api/grpcclient/interceptor.go` - 拦截器实现
- `api/grpcclient/client.go` - 拦截器集成

拦截器会检查当前日志级别，仅在 DEBUG 级别时才记录日志，不会影响其他级别的性能。
