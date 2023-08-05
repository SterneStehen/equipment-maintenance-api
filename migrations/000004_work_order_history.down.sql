BEGIN;

DROP TABLE work_order_history;

UPDATE work_orders
SET status = 'completed',
    completed_at = COALESCE(completed_at, updated_at, NOW())
WHERE status = 'closed';

ALTER TABLE work_orders DROP CONSTRAINT work_orders_status_valid;
ALTER TABLE work_orders ADD CONSTRAINT work_orders_status_valid
    CHECK (status IN ('open', 'in_progress', 'completed', 'canceled'));

ALTER TABLE work_orders DROP CONSTRAINT work_orders_completed_time;
ALTER TABLE work_orders ADD CONSTRAINT work_orders_completed_time CHECK (
    (status = 'completed' AND completed_at IS NOT NULL)
    OR (status <> 'completed' AND completed_at IS NULL)
);

COMMIT;
