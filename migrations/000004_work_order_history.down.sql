BEGIN;

DROP TABLE work_order_history;

UPDATE work_orders
SET status = 'completed'
WHERE status = 'closed';

ALTER TABLE work_orders DROP CONSTRAINT work_orders_status_valid;
ALTER TABLE work_orders ADD CONSTRAINT work_orders_status_valid
    CHECK (status IN ('open', 'in_progress', 'completed', 'canceled'));

COMMIT;
