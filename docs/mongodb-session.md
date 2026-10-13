# MongoDB SessionService

实现参考当前依赖 `google.golang.org/adk/v2 v2.0.0` 的 `session/database`。
业务集合 `session` / `message` 与 ADK 存储分开，原业务文档不需要迁移。
映射定义在 `internal/store/schema/adk_session.go`。

适配层按职责拆分：`SessionService` 负责参数校验和请求分发；`localSession` 负责会话
创建、加载及提交后的更新；`localState` / `stateDelta` 负责状态读写、分区和编码；
`sessionEvent` / `localEvents` 负责事件序列化、快照和提交后回写；`sessionList` 负责
用户去重及列表装配。会话、状态、事件分别管理自身数据和锁，对象之间通过方法协作。

## 手动初始化

部署前在业务数据库中执行以下 mongosh 命令。代码不会执行集合、索引创建或迁移。
数据库必须是支持多文档事务的 replica set（开发环境可使用单节点 replica set）
或 sharded cluster，standalone 不支持此实现的写入。
参见 [MongoDB 事务文档](https://www.mongodb.com/docs/manual/core/transactions/)。

```javascript
// 先通过 use <数据库名> 切换到传入 store.New(client, database) 的数据库。
// 新集合执行 createCollection；已经存在时跳过对应命令。
db.createCollection("adk_session");
db.createCollection("adk_session_event");
db.createCollection("adk_app_state");
db.createCollection("adk_user_state");

// 同时支持按 app 列表、按 app + user 列表及精确会话查找。
db.adk_session.createIndex(
  { app_name: 1, user_id: 1, session_id: 1 },
  { unique: true, name: "uq_adk_session_identity" }
);

// 保证同一个会话不能重复写入同一个事件 ID。
db.adk_session_event.createIndex(
  { session: 1, event_id: 1 },
  { unique: true, name: "uq_adk_session_event_id" }
);

// 支持历史读取、After 过滤、最新 N 条及相同时间戳下的稳定排序；
// session 前缀也支持删除该会话的全部事件。
db.adk_session_event.createIndex(
  { session: 1, timestamp: -1, sequence: -1 },
  { name: "ix_adk_session_event_history" }
);

db.adk_app_state.createIndex(
  { app_name: 1 },
  { unique: true, name: "uq_adk_app_state" }
);
db.adk_user_state.createIndex(
  { app_name: 1, user_id: 1 },
  { unique: true, name: "uq_adk_user_state" }
);
```

上述唯一索引是正确性要求，必须在启用服务前完成。已有 ADK 数据时先处理重复记录。
分片部署需自行选择 shard key，并确保唯一索引包含 shard key 前缀；以上命令可直接用于
replica set 或 sharded cluster 中的未分片集合。

## 存储和一致性

- `adk_session`：会话标识、会话状态、版本号、创建与更新时间；不嵌入历史事件。
- `adk_session_event`：通过 ObjectID 关联会话，保存事件 ID、版本序号、时间和完整 JSON
  payload（BSON binary），保留工具调用、二进制内容、元数据和工作流字段。
- `adk_app_state` / `adk_user_state`：分别保存应用与用户共享状态，仅在有对应 delta 时写入。
- 状态字段 `state` 为文档，键采用 `"k" + base64url(原始键)`（无 padding），值为 JSON
  编码的 BSON binary。这样 `.`、`$`、空键等均能安全使用 `$set`，并沿用 ADK database
  service 的 JSON 值语义（读取后的数字为 float64，nil 为 JSON null）。
- `app:` / `user:` 键移入对应共享状态，读取时恢复前缀；`temp:` 初始状态不保存，事件的
  `temp:` delta 仅更新本次调用的本地状态，不保存在事件中。partial 事件不写入或更新状态。
- `State().Set` 仅改变本地快照；持久化由 `EventActions.StateDelta` 驱动，与 ADK 一致。
- 写事务使用 snapshot read concern、majority write concern 和 primary read preference。
  每次 append 使用 `_id + version` 条件更新，防止过期快照覆盖；冲突后调用 Get 重新读取，
  不要自动用旧事件覆盖新状态。同一毫秒内写入也能检测冲突。
- 只有事务提交成功才更新本地会话及调用者事件。删除事务同时删除会话和全部关联事件，
  保留共享状态；删除不存在的会话成功返回。重新使用公共 SessionID 时会分配新 ObjectID。
- 时间统一为 UTC，按 BSON Date 的毫秒精度截断；同一时间戳按事件序号排序。
  没有事件 ID 或时间戳时，通过 ADK platform 生成。

## 性能边界

普通 append 执行两条数据写命令（版本/状态更新及事件插入），没有共享状态查询；
仅在对应 delta 非空时增加共享状态 `$set` upsert。所有值在事务外序列化，事务回调中
不更改本地状态，可以安全使用驱动的事务重试。

Get 在 MongoDB 内执行时间过滤、倒序排序及 limit，再在内存中反转为时间正序。
事件查询限定为读取到的会话版本，避免返回并发 append 后的新事件与旧会话状态混用。
无事件的会话跳过历史查询。正常读取不启用多文档事务，因此共享状态是各查询时的最新
快照，不保证 Get/List 的所有集合来自同一个数据库快照。

List 不读事件，应用状态仅查询一次；用户去重后使用每批最多 128 个用户的 `$in` 查询，
只读取列表涉及的用户状态，避免 N+1 查询和读取所有应用用户。MongoDB cursor 采用
128 条批次并关闭，不执行 count、skip 或索引检查。

ADK 接口的 `NumRecentEvents == 0` 表示完整历史，List 接口也没有分页字段，因此这些
调用仍可能返回大量数据，不能在实现中截断而破坏接口语义。历史较长时调用方应设置
`NumRecentEvents` / `After`，列表优先指定 `UserID`。避免频繁修改 `app:` 状态，所有用户
写入同一应用状态文档时会竞争。清理超大会话会产生与事件数相应的删除工作量，应在
低峰执行；服务不会自动设置 TTL 或运行后台清理任务。

## 验证

```powershell
go test ./...
go test -race ./internal/service/adapter ./internal/store/...
```

新增测试使用 MongoDB 驱动的 mock deployment，验证实际生成的查询、事务提交/回滚命令、
状态分区、事件字段 round trip、过滤与批量读取，不连接真实数据库或执行 DDL。
