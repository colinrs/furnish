-- 每个上下文的表用前缀区分,方便将来拆成独立服务和独立数据库。

CREATE TABLE ordering_orders (
    id            TEXT PRIMARY KEY,
    customer_id   TEXT        NOT NULL,
    status        TEXT        NOT NULL,
    credit_limit  BIGINT      NOT NULL,
    currency      CHAR(3)     NOT NULL,
    version       INT         NOT NULL,          -- 乐观锁
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ordering_order_lines (
    order_id    TEXT   NOT NULL REFERENCES ordering_orders(id) ON DELETE CASCADE,
    line_no     INT    NOT NULL,
    product_id  TEXT   NOT NULL,
    unit_price  BIGINT NOT NULL,                 -- 下单时的价格快照(分)
    quantity    INT    NOT NULL CHECK (quantity BETWEEN 1 AND 20),
    PRIMARY KEY (order_id, line_no)
);

CREATE TABLE inventory_stocks (
    sku       TEXT PRIMARY KEY,
    on_hand   INT  NOT NULL,
    reserved  INT  NOT NULL DEFAULT 0 CHECK (reserved <= on_hand),
    version   INT  NOT NULL
);

-- 事务性发件箱
CREATE TABLE outbox (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    published_at  TIMESTAMPTZ
);
CREATE INDEX outbox_unpublished ON outbox (id) WHERE published_at IS NULL;

-- 消费端幂等
CREATE TABLE processed_messages (
    consumer  TEXT NOT NULL,
    key       TEXT NOT NULL,
    PRIMARY KEY (consumer, key)
);
