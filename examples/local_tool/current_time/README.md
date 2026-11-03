# current_time local tool

此示例遵循 internal/toolx/local 的加载与执行协议：

- tool.json 使用 parametersJsonSchema 和可选的 responseJsonSchema。
- 注册名称、manifest 的 name 和可执行文件名一致：current_time（Windows 加 .exe）。
- 每次调用启动一个进程，从 stdin 读取 JSON 参数，stdout 只输出一个 JSON 对象。
- 输入错误以 error 字段返回并正常退出，使服务能够读取错误详情；进程故障才使用非零退出码。日志写入 stderr。
- timezone 可省略，默认 Asia/Shanghai。时区数据库内嵌于二进制。

## 编译和验证

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o current_time .

tar -czf current_time-linux-amd64.tar.gz tool.json current_time
```

## 接入服务

将压缩包放到服务可下载的 HTTP(S) 地址，通过服务现有认证方式调用 POST /v1/tool（如部署有额外路由前缀，需补上），请求体如下：

```json
{
  "name": "current_time",
  "kind": 0,
  "url": "https://your-host.example/current_time-windows.zip",
  "agent": "替换为实际的 agent ID"
}
```

URL 必须替换为实际压缩包地址，agent 必须替换为目标 Agent ID。服务异步下载并安装，安装成功后状态为 online；创建接口成功不代表安装已成功。安装内容保存到 tools/current_time/<version>/。
