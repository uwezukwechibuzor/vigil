{{
  config(materialized = 'table')
}}

-- Activation funnel and time-to-first-value by signup week.
-- grain: one row per org
-- Definition: see macros/activation.sql (versioned)
with orgs as (
    select
        org_id,
        plan,
        signup_date,
        date_trunc('week', signup_date)   as signup_week,
        signup_channel,
        first_framework,
        first_chain,
        is_activated,
        days_to_first_value,
        is_paid
    from {{ ref('dim_orgs') }}
)

select
    signup_week,
    plan,
    signup_channel,
    first_framework,
    count(*)                             as orgs_signed_up,
    sum(case when is_activated then 1 else 0 end) as orgs_activated,
    round(
        cast(sum(case when is_activated then 1 else 0 end) as double)
        / nullif(count(*), 0),
        4
    )                                    as activation_rate,
    round(avg(case when is_activated then days_to_first_value end), 1)
                                         as avg_days_to_first_value,
    round(median(case when is_activated then days_to_first_value end), 1)
                                         as median_days_to_first_value,
    sum(case when is_paid then 1 else 0 end) as orgs_paid,
    round(
        cast(sum(case when is_paid then 1 else 0 end) as double)
        / nullif(count(*), 0),
        4
    )                                    as paid_conversion_rate
from orgs
group by signup_week, plan, signup_channel, first_framework
order by signup_week desc
