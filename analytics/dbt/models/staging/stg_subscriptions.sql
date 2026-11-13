{{
  config(materialized = 'view')
}}

select
    sub_id,
    org_id,
    lower(trim(plan))             as plan,
    lower(trim(status))           as status,
    cast(started_at as timestamp) as started_at,
    cast(started_at as date)      as started_date,
    nullif(ended_at, '')          as ended_at,
    mrr_cents,
    nullif(canceled_reason, '')   as canceled_reason
from {{ ref('seed_subscriptions') }}
where sub_id is not null
