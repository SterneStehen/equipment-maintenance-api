BEGIN;

CREATE TABLE audit_events (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT REFERENCES users(id),
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id BIGINT,
    details TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT audit_events_action_not_empty CHECK (BTRIM(action) <> ''),
    CONSTRAINT audit_events_target_type_not_empty CHECK (BTRIM(target_type) <> '')
);

CREATE INDEX audit_events_actor_id_idx ON audit_events (actor_id);
CREATE INDEX audit_events_target_idx ON audit_events (target_type, target_id);
CREATE INDEX audit_events_created_at_idx ON audit_events (created_at);

COMMIT;
