{{
  config(materialized = 'view')
}}

select
    project_id,
    org_id,
    name,
    cast(created_at as timestamp) as created_at,
    nullif(deleted_at, '')        as deleted_at
from {{ ref('seed_projects') }}
