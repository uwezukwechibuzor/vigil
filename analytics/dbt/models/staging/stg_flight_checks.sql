{{
  config(materialized = 'view')
}}

select
    check_id,
    org_id,
    project_id,
    lower(trim(chain))            as chain,
    lower(trim(verdict))          as verdict,
    cast(risk_score as double)    as risk_score,
    cast(latency_ms as integer)   as latency_ms,
    cast(checks_failed as integer) as checks_failed,
    cast(occurred_at as timestamp) as occurred_at,
    cast(occurred_at as date)      as occurred_date
from {{ ref('seed_flight_checks') }}
where check_id is not null
