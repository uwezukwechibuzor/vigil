{{
  config(materialized = 'table')
}}

-- Monthly MRR waterfall: new, expansion, contraction, churn, reactivation.
-- grain: one row per (month, mrr_movement)
select
    date_trunc('month', cast(started_at as date)) as month,
    mrr_movement,
    count(distinct org_id)              as orgs,
    sum(mrr_cents)                      as gross_mrr_cents,
    sum(mrr_delta_cents)                as net_mrr_delta_cents,
    round(avg(mrr_cents), 0)            as avg_mrr_cents
from {{ ref('fct_subscriptions') }}
where mrr_movement != 'unchanged'
group by date_trunc('month', cast(started_at as date)), mrr_movement
order by month, mrr_movement
