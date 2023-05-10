BEGIN;

CREATE TABLE work_orders (
    id BIGSERIAL PRIMARY KEY,
    equipment_id BIGINT NOT NULL REFERENCES equipment(id),
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    priority TEXT NOT NULL DEFAULT 'medium',
    assigned_to BIGINT REFERENCES users(id),
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT work_orders_title_not_empty CHECK (BTRIM(title) <> ''),
    CONSTRAINT work_orders_status_valid CHECK (status IN ('open', 'in_progress', 'completed', 'canceled')),
    CONSTRAINT work_orders_priority_valid CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    CONSTRAINT work_orders_completed_time CHECK (
        (status = 'completed' AND completed_at IS NOT NULL)
        OR (status <> 'completed' AND completed_at IS NULL)
    )
);

CREATE INDEX work_orders_equipment_id_idx ON work_orders (equipment_id);
CREATE INDEX work_orders_status_idx ON work_orders (status);
CREATE INDEX work_orders_assigned_to_idx ON work_orders (assigned_to);
CREATE INDEX work_orders_created_at_idx ON work_orders (created_at);

COMMIT;
