BEGIN;

INSERT INTO sites(id, code, name, region, state, version, created_at, updated_at)
VALUES (
    'site_demo', 'SITE-DEMO', '滨江演示场地', '华东', 'active', 1,
    '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO facilities(
    id, site_id, code, name, kind, critical, enabled, version, created_at, updated_at
)
VALUES (
    'facility_demo', 'site_demo', 'POWER-01', '一号配电设施', 'electrical', true, true, 1,
    '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO risk_categories(
    id, code, name, base_weight, default_sla_minutes, description, version, created_at
)
VALUES (
    'risk_electrical', 'ELECTRICAL', '电气安全', 72, 1440,
    '配电、漏电、短路与过载风险', 1, '2026-01-01T00:00:00Z'
)
ON CONFLICT (id) DO NOTHING;

COMMIT;
