BEGIN;

DO $safety_types$
BEGIN
    IF to_regtype('safety_revision') IS NULL THEN
        EXECUTE 'CREATE DOMAIN safety_revision AS bigint CHECK (VALUE > 0)';
    END IF;
    IF to_regtype('safety_key') IS NULL THEN
        EXECUTE 'CREATE DOMAIN safety_key AS text CHECK ('
            || 'VALUE = btrim(VALUE) AND char_length(VALUE) BETWEEN 1 AND 180)';
    END IF;
    IF to_regtype('safety_document') IS NULL THEN
        EXECUTE 'CREATE DOMAIN safety_document AS jsonb CHECK ('
            || 'jsonb_typeof(VALUE) = ''object'')';
    END IF;
    IF to_regtype('safety_queue') IS NULL THEN
        EXECUTE 'CREATE TYPE safety_queue AS ENUM (''normal'', ''focus'')';
    END IF;
    IF to_regtype('safety_hazard_state') IS NULL THEN
        EXECUTE 'CREATE TYPE safety_hazard_state AS ENUM ('
            || '''pending_confirmation'', ''handling'', ''pending_review'', '
            || '''resolved'', ''closed'')';
    END IF;
    IF to_regtype('safety_risk_level') IS NULL THEN
        EXECUTE 'CREATE TYPE safety_risk_level AS ENUM ('
            || '''low'', ''medium'', ''high'', ''critical'')';
    END IF;
END
$safety_types$;

CREATE TABLE IF NOT EXISTS sites (
    id safety_key PRIMARY KEY,
    code safety_key NOT NULL UNIQUE,
    name text NOT NULL CHECK (name = btrim(name) AND name <> ''),
    region text NOT NULL,
    state text NOT NULL,
    version safety_revision NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS facilities (
    id safety_key PRIMARY KEY,
    site_id safety_key NOT NULL REFERENCES sites(id),
    code safety_key NOT NULL,
    name text NOT NULL CHECK (name = btrim(name) AND name <> ''),
    kind text NOT NULL,
    critical boolean NOT NULL,
    enabled boolean NOT NULL,
    version safety_revision NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE(site_id, code)
);

CREATE TABLE IF NOT EXISTS risk_categories (
    id safety_key PRIMARY KEY,
    code safety_key NOT NULL UNIQUE,
    name text NOT NULL CHECK (name = btrim(name) AND name <> ''),
    base_weight integer NOT NULL CHECK (base_weight BETWEEN 0 AND 100),
    default_sla_minutes integer NOT NULL CHECK (default_sla_minutes > 0),
    description text NOT NULL,
    version safety_revision NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS escalation_rule_versions (
    rule_id safety_key NOT NULL,
    version safety_revision NOT NULL,
    name text NOT NULL CHECK (name = btrim(name) AND name <> ''),
    description text NOT NULL,
    enabled boolean NOT NULL,
    priority integer NOT NULL,
    condition safety_document NOT NULL,
    focus_sla_millis bigint NOT NULL CHECK (focus_sla_millis > 0),
    notify_after_millis bigint NOT NULL CHECK (notify_after_millis >= 0),
    renotify_every_millis bigint NOT NULL CHECK (renotify_every_millis > 0),
    created_by safety_key NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY(rule_id, version)
);

CREATE TABLE IF NOT EXISTS inspections (
    id safety_key PRIMARY KEY,
    site_id safety_key NOT NULL REFERENCES sites(id),
    facility_id safety_key NOT NULL REFERENCES facilities(id),
    idempotency_key safety_key NOT NULL UNIQUE,
    created_at timestamptz NOT NULL,
    data safety_document NOT NULL
);

CREATE TABLE IF NOT EXISTS hazards (
    id safety_key PRIMARY KEY,
    idempotency_key safety_key NOT NULL UNIQUE,
    site_id safety_key NOT NULL REFERENCES sites(id),
    queue safety_queue NOT NULL,
    state safety_hazard_state NOT NULL,
    risk_level safety_risk_level NOT NULL,
    risk_score integer NOT NULL CHECK (risk_score BETWEEN 0 AND 100),
    due_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    version safety_revision NOT NULL,
    data safety_document NOT NULL
);

CREATE INDEX IF NOT EXISTS hazards_queue_due_idx ON hazards(queue, due_at);
CREATE INDEX IF NOT EXISTS hazards_site_state_idx ON hazards(site_id, state);
CREATE INDEX IF NOT EXISTS hazards_created_idx ON hazards(created_at);

CREATE TABLE IF NOT EXISTS evidence (
    id safety_key PRIMARY KEY,
    hazard_id safety_key NOT NULL REFERENCES hazards(id) ON DELETE CASCADE,
    storage_key safety_key NOT NULL,
    created_at timestamptz NOT NULL,
    data safety_document NOT NULL,
    UNIQUE(hazard_id, storage_key)
);

CREATE TABLE IF NOT EXISTS queue_entries (
    hazard_id safety_key PRIMARY KEY REFERENCES hazards(id) ON DELETE CASCADE,
    queue safety_queue NOT NULL,
    owner_id text NOT NULL DEFAULT '',
    due_at timestamptz NOT NULL,
    next_notify_at timestamptz,
    acknowledged_at timestamptz,
    version safety_revision NOT NULL,
    data safety_document NOT NULL
);

CREATE INDEX IF NOT EXISTS queue_entries_notify_idx ON queue_entries(next_notify_at) WHERE acknowledged_at IS NULL;

CREATE TABLE IF NOT EXISTS remediation_plans (
    id safety_key PRIMARY KEY,
    hazard_id safety_key NOT NULL UNIQUE REFERENCES hazards(id) ON DELETE CASCADE,
    version safety_revision NOT NULL,
    data safety_document NOT NULL
);

CREATE TABLE IF NOT EXISTS reinspections (
    id safety_key PRIMARY KEY,
    hazard_id safety_key NOT NULL REFERENCES hazards(id) ON DELETE CASCADE,
    idempotency_key safety_key NOT NULL UNIQUE,
    completed_at timestamptz NOT NULL,
    data safety_document NOT NULL
);

CREATE TABLE IF NOT EXISTS notifications (
    id safety_key PRIMARY KEY,
    hazard_id safety_key NOT NULL REFERENCES hazards(id) ON DELETE CASCADE,
    deduplication_key safety_key NOT NULL UNIQUE,
    sent_at timestamptz,
    data safety_document NOT NULL
);

CREATE TABLE IF NOT EXISTS notification_attempts (
    id bigserial PRIMARY KEY,
    notification_id safety_key NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    attempted_at timestamptz NOT NULL,
    data safety_document NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_events (
    id safety_key PRIMARY KEY,
    hazard_id safety_key REFERENCES hazards(id) ON DELETE CASCADE,
    site_id safety_key REFERENCES sites(id),
    event_type safety_key NOT NULL,
    occurred_at timestamptz NOT NULL,
    data safety_document NOT NULL
);

CREATE INDEX IF NOT EXISTS audit_events_hazard_time_idx ON audit_events(hazard_id, occurred_at, id);

COMMIT;
