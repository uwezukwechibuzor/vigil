{{
  config(
    materialized = 'view',
    description  = 'Cleaned, deduplicated event stream. Source of truth for all behavioural analytics.'
  )
}}

with raw as (
    select * from {{ ref('seed_events') }}
),

deduped as (
    select *,
        row_number() over (partition by event_id order by received_at) as rn
    from raw
    where event_id is not null
      and event_name is not null
),

cleaned as (
    select
        event_id,
        org_id,
        nullif(project_id, '')  as project_id,
        nullif(user_id, '')     as user_id,
        anonymous_id,
        nullif(session_id, '')  as session_id,
        cast(occurred_at as timestamp) as occurred_at,
        cast(received_at as timestamp) as received_at,
        lower(trim(surface))    as surface,
        lower(trim(event_name)) as event_name,
        app_version,
        lower(trim(plan))       as plan,
        lower(trim(environment)) as environment,
        lower(trim(framework))  as framework,
        nullif(chain_id, '')    as chain_id,
        properties
    from deduped
    where rn = 1
      -- drop internal orgs and test events
      and org_id not in (select org_id from {{ ref('seed_orgs') }} where is_internal = true)
      and environment != 'development'
)

select * from cleaned
