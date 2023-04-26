BEGIN;

CREATE TABLE equipment (
    id BIGSERIAL PRIMARY KEY,
    serial_number TEXT NOT NULL,
    name TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decommissioned_at TIMESTAMPTZ,
    CONSTRAINT equipment_serial_not_empty CHECK (serial_number <> ''),
    CONSTRAINT equipment_serial_normalized CHECK (serial_number = UPPER(BTRIM(serial_number))),
    CONSTRAINT equipment_serial_unique UNIQUE (serial_number),
    CONSTRAINT equipment_name_not_empty CHECK (BTRIM(name) <> ''),
    CONSTRAINT equipment_status_valid CHECK (status IN ('active', 'maintenance', 'decommissioned')),
    CONSTRAINT equipment_decommission_time CHECK (
        (status = 'decommissioned' AND decommissioned_at IS NOT NULL)
        OR (status <> 'decommissioned' AND decommissioned_at IS NULL)
    )
);

CREATE INDEX equipment_status_idx ON equipment (status);
CREATE INDEX equipment_serial_number_idx ON equipment (serial_number);

COMMIT;
