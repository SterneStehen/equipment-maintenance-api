BEGIN;

ALTER TABLE work_orders DROP CONSTRAINT work_orders_status_valid;
ALTER TABLE work_orders ADD CONSTRAINT work_orders_status_valid
    CHECK (status IN ('open', 'in_progress', 'completed', 'closed', 'canceled'));

ALTER TABLE work_orders DROP CONSTRAINT work_orders_completed_time;
ALTER TABLE work_orders ADD CONSTRAINT work_orders_completed_time CHECK (
    (status IN ('completed', 'closed') AND completed_at IS NOT NULL)
    OR (status NOT IN ('completed', 'closed') AND completed_at IS NULL)
);

CREATE TABLE work_order_history (
    id BIGSERIAL PRIMARY KEY,
    work_order_id BIGINT NOT NULL REFERENCES work_orders(id),
    from_status TEXT NOT NULL,
    to_status TEXT NOT NULL,
    actor_id BIGINT NOT NULL REFERENCES users(id),
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT work_order_history_from_status_valid CHECK (from_status IN ('open', 'in_progress', 'completed', 'closed', 'canceled')),
    CONSTRAINT work_order_history_to_status_valid CHECK (to_status IN ('open', 'in_progress', 'completed', 'closed', 'canceled'))
);

CREATE INDEX work_order_history_work_order_id_idx ON work_order_history (work_order_id);

COMMIT;
