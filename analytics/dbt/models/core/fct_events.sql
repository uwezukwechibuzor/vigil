{{
  config(materialized = 'table')
}}

select
    e.event_id,
    e.org_id,
    e.project_id,
    e.user_id,
    e.anonymous_id,
    e.session_id,
    e.occurred_at,
    e.received_at,
    cast(e.occurred_at as date)   as event_date,
    date_trunc('week', cast(e.occurred_at as date)) as event_week,
    e.surface,
    e.event_name,
    e.app_version,
    e.plan,
    e.environment,
    e.framework,
    e.chain_id,
    e.properties,
    -- join org dimension for enrichment
    o.signup_date,
    o.signup_channel,
    o.is_activated,
    o.is_paid
from {{ ref('stg_events') }} e
left join {{ ref('dim_orgs') }} o using (org_id)
