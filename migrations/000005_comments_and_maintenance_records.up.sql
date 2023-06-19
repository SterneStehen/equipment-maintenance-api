BEGIN;

CREATE TABLE work_order_comments (
    id BIGSERIAL PRIMARY KEY,
    work_order_id BIGINT NOT NULL REFERENCES work_orders(id),
    author_id BIGINT NOT NULL REFERENCES users(id),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT work_order_comments_body_not_empty CHECK (BTRIM(body) <> '')
);

CREATE INDEX work_order_comments_work_order_id_idx ON work_order_comments (work_order_id);
CREATE INDEX work_order_comments_created_at_idx ON work_order_comments (created_at);

CREATE TABLE maintenance_records (
    id BIGSERIAL PRIMARY KEY,
    work_order_id BIGINT NOT NULL UNIQUE REFERENCES work_orders(id),
    equipment_id BIGINT NOT NULL REFERENCES equipment(id),
    performed_by BIGINT NOT NULL REFERENCES users(id),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX maintenance_records_equipment_id_idx ON maintenance_records (equipment_id);
CREATE INDEX maintenance_records_performed_by_idx ON maintenance_records (performed_by);

COMMIT;
