{{
  config(materialized = 'table')
}}

-- Daily, weekly and monthly active org counts.
-- grain: one row per event_date
with daily as (
    select
        event_date,
        count(distinct org_id)                       as dau,
        count(distinct case when surface != 'backend' then org_id end) as frontend_dau,
        count(*)                                     as event_count
    from {{ ref('fct_events') }}
    group by event_date
),

rolling as (
    select
        event_date,
        dau,
        frontend_dau,
        event_count,
        sum(dau) over (order by event_date rows between 6 preceding and current row)  as wau_approx,
        sum(dau) over (order by event_date rows between 29 preceding and current row) as mau_approx
    from daily
)

select
    event_date,
    dau,
    frontend_dau,
    event_count,
    wau_approx,
    mau_approx,
    case
        when mau_approx > 0
        then round(cast(dau as double) / cast(mau_approx as double), 4)
    end as stickiness_ratio
from rolling
order by event_date
