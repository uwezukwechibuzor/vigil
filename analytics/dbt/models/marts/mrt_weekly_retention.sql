{{
  config(materialized = 'table')
}}

-- Weekly retention by signup cohort.
-- grain: one row per (signup_week, activity_week)
-- stickiness: % of signup-week orgs that were active N weeks later

with org_weeks as (
    select distinct
        org_id,
        event_week
    from {{ ref('fct_events') }}
),

cohorts as (
    select
        org_id,
        date_trunc('week', signup_date) as cohort_week
    from {{ ref('dim_orgs') }}
),

joined as (
    select
        c.cohort_week,
        ow.event_week,
        datediff('week', cast(c.cohort_week as date), cast(ow.event_week as date)) as weeks_since_signup,
        c.org_id
    from cohorts c
    inner join org_weeks ow using (org_id)
    where ow.event_week >= c.cohort_week
),

cohort_sizes as (
    select cohort_week, count(distinct org_id) as cohort_size
    from cohorts
    group by cohort_week
)

select
    j.cohort_week,
    j.weeks_since_signup,
    count(distinct j.org_id)                      as retained_orgs,
    cs.cohort_size,
    round(
        cast(count(distinct j.org_id) as double) / nullif(cs.cohort_size, 0),
        4
    )                                              as retention_rate
from joined j
inner join cohort_sizes cs using (cohort_week)
where j.weeks_since_signup <= 12
group by j.cohort_week, j.weeks_since_signup, cs.cohort_size
order by j.cohort_week, j.weeks_since_signup
