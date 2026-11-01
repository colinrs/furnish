# furnish:Go DDD 示例仓库

一个家具电商的**模块化单体**示例,用来演示 DDD 项目在 Go + PostgreSQL + Redis + GORM 下的目录结构和依赖规则。

包含两个限界上下文:

- **ordering(下单)**:完整示例。聚合、值对象、领域事件、仓储、应用服务、防腐层、HTTP、GORM、Redis 幂等。
- **inventory(库存)**:通过事件订阅下单上下文,演示最终一致性和消费端幂等。

## 目录结构

```
furnish/
├── cmd/api/main.go                 组装根:创建 DB/Redis,接线各上下文,启动 HTTP 和 outbox relay
├── internal/
│   ├── ordering/                   ← 限界上下文:下单
│   │   ├── domain/                 领域层,纯 Go,不 import gorm / redis / net/http
│   │   │   ├── order.go            聚合根 Order + 内部实体 OrderLine + 工厂 PlaceOrder
│   │   │   ├── money.go            值对象 Money(不可变,闭合操作)
│   │   │   ├── events.go           领域事件 OrderPlaced / OrderLocked …
│   │   │   ├── repository.go       仓储接口(只为聚合根)
│   │   │   ├── errors.go           领域错误
│   │   │   └── order_test.go       不变量测试(表驱动)
│   │   ├── app/                    应用层:用例、事务边界、事件转发布语言
│   │   │   ├── ports.go            端口:Catalog、CreditPolicy、Transactor、EventRecorder、IdempotencyGuard
│   │   │   ├── service.go          PlaceOrder / ChangeQuantity / Lock
│   │   │   └── service_test.go     用内存替身测试,不需要数据库
│   │   ├── contract/               发布语言:对其他上下文公开的事件结构(带版本号)
│   │   ├── adapter/                适配器层(六边形的外圈)
│   │   │   ├── http/               入站:HTTP handler,领域错误 → 状态码
│   │   │   ├── postgres/           出站:GORM 持久化模型 + 映射 + 仓储实现 + 乐观锁
│   │   │   ├── redis/              出站:幂等键(SETNX)
│   │   │   ├── catalogacl/         出站:防腐层,翻译老 ERP 的物料号和金额
│   │   │   └── credit/             出站:信用额度
│   │   └── module.go               上下文的唯一组装入口
│   ├── inventory/                  ← 限界上下文:库存
│   │   ├── domain/                 Stock 聚合(一个 SKU 一个小聚合)
│   │   ├── app/                    OnOrderPlaced:订阅 ordering/contract 事件,预留库存
│   │   └── adapter/memory/         内存实现(演示用)
│   └── platform/                   技术基础设施,不含业务
│       ├── tx/                     基于 context 传递的 GORM 事务
│       └── outbox/                 事务性发件箱 + relay(FOR UPDATE SKIP LOCKED)
├── migrations/0001_init.sql        表按上下文加前缀
└── .golangci.yml                   depguard 规则,守住依赖方向
```

## 依赖规则

```
adapter ──→ app ──→ domain
   │                  ↑
   └──────────────────┘     (adapter 实现 domain/app 定义的接口)

其他上下文 ──→ ordering/contract     (只能依赖发布语言)
```

1. `domain` 只依赖标准库。
2. `app` 依赖 `domain`,通过 `ports.go` 里的接口访问外部世界。
3. `adapter` 实现这些接口,可以使用 gorm、redis、net/http。
4. 上下文之间只能依赖对方的 `contract` 包,协作靠事件(或对方公开的接口)。
5. Go 的 `internal/` 挡不住同一模块内的上下文互相 import,所以用 `.golangci.yml` 里的 depguard 规则在 CI 中检查。

## 关键设计点

| 主题 | 位置 | 说明 |
|---|---|---|
| 聚合不变量 | `ordering/domain/order.go` | 字段未导出,只能通过方法修改;违反"总额 ≤ 信用额度"时回滚并返回错误 |
| 价格快照 | `OrderLine.unitPrice` | 订单行保存下单时的价格,商品调价不影响历史订单 |
| 持久化模型分离 | `adapter/postgres` | `orderPO` 带 GORM 标签,`domain.Order` 不知道数据库的存在 |
| 乐观锁 | `Repository.Save` | `UPDATE … WHERE id = ? AND version = ?`,影响 0 行返回 `ErrConcurrentModification` |
| 一个事务一个聚合 | `app/service.go` | 事务内只保存一个 Order,并把事件写入 outbox |
| 最终一致性 | `platform/outbox` + `inventory/app` | 下单提交后,库存稍后预留;按"事件+SKU"去重,重复投递无害 |
| 防腐层 | `adapter/catalogacl` | 老 ERP 的 MATNR、字符串金额、状态码,在这里翻译成 ProductID 和 Money |
| Redis 的角色 | `adapter/redis` | 只做幂等、缓存等辅助;事实来源是 PostgreSQL |

## 运行

```bash
go mod tidy                      # 生成 go.sum(示例仓库未附带)
go test ./...                    # domain / app / acl / inventory 的测试不需要数据库
psql "$DATABASE_URL" -f migrations/0001_init.sql
DATABASE_URL=postgres://... REDIS_ADDR=localhost:6379 go run ./cmd/api

curl -X POST localhost:8080/orders -H 'Idempotency-Key: abc' \
  -d '{"customer_id":"C-1","items":[{"product_id":"SOFA-OAT-3S","quantity":1}]}'
```

## 下一步可以练习

- 给 inventory 写 GORM 仓储和 `processed_messages` 实现,把预留和"已处理"标记放进同一事务。
- 库存不足时发出 `StockShortage` 事件,让 ordering 订阅它并自动锁单(一个简单的 Saga)。
- 新增 `delivery` 上下文,订阅 `OrderPlaced` 生成配送任务。
- 为订单列表做一个读模型(CQRS):订阅事件写入一张宽表或 Redis,查询不走聚合。
