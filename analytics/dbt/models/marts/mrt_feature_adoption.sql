{{
  config(materialized = 'table')
}}

-- Feature adoption: % of active orgs using each event/feature per week.
-- grain: one row per (event_week, event_name)
with weekly_active as (
    select
        event_week,
        count(distinct org_id) as weekly_active_orgs
    from {{ ref('fct_events') }}
    group by event_week
),

feature_usage as (
    select
        event_week,
        event_name,
        count(distinct org_id) as orgs_using_feature,
        count(*)               as event_count
    from {{ ref('fct_events') }}
    group by event_week, event_name
)

select
    fu.event_week,
    fu.event_name,
    fu.orgs_using_feature,
    fu.event_count,
    wa.weekly_active_orgs,
    round(
        cast(fu.orgs_using_feature as double) / nullif(wa.weekly_active_orgs, 0),
        4
    ) as adoption_rate
from feature_usage fu
inner join weekly_active wa using (event_week)
order by fu.event_week desc, adoption_rate desc
