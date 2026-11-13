{{
  config(materialized = 'view')
}}

select
    org_id,
    name,
    lower(trim(plan))             as plan,
    cast(created_at as timestamp) as created_at,
    cast(created_at as date)      as signup_date,
    lower(trim(signup_channel))   as signup_channel,
    lower(trim(first_framework))  as first_framework,
    lower(trim(first_chain))      as first_chain,
    is_internal,
    nullif(deleted_at, '')        as deleted_at
from {{ ref('seed_orgs') }}
where is_internal = false
