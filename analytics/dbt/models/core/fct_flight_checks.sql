{{
  config(materialized = 'table')
}}

select
    fc.check_id,
    fc.org_id,
    fc.project_id,
    fc.chain,
    fc.verdict,
    fc.risk_score,
    fc.latency_ms,
    fc.checks_failed,
    fc.occurred_at,
    fc.occurred_date,
    date_trunc('week', fc.occurred_date) as occurred_week,
    -- enrichment from dim
    o.plan,
    o.signup_channel,
    o.first_framework,
    o.is_activated,
    o.is_paid
from {{ ref('stg_flight_checks') }} fc
left join {{ ref('dim_orgs') }} o using (org_id)
