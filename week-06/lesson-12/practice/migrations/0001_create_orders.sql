-- Миграция 0001: таблицы заказов. ГОТОВО, менять не нужно.
-- Контракт CAPSTONE.md, неделя 6: orders (id text PK, customer_id, status,
-- total bigint, created_at timestamptz). Позиции храним отдельной
-- таблицей order_items (схема зафиксирована в README практики урока 11).

-- +goose Up
CREATE TABLE orders (
    id          text PRIMARY KEY,
    customer_id text NOT NULL,
    status      text NOT NULL,
    total       bigint NOT NULL,
    created_at  timestamptz NOT NULL
);

CREATE TABLE order_items (
    order_id   text NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id text NOT NULL,
    name       text NOT NULL,
    quantity   integer NOT NULL,
    price      bigint NOT NULL
);

-- +goose Down
DROP TABLE order_items;
DROP TABLE orders;
