{{
  config(materialized = 'table')
}}

-- MRR movement categories: new, expansion, contraction, churn, reactivation
with current_subs as (
    select
        sub_id,
        org_id,
        plan,
        status,
        started_at,
        started_date,
        ended_at,
        mrr_cents,
        canceled_reason,
        lag(plan) over (partition by org_id order by started_at) as prev_plan,
        lag(mrr_cents) over (partition by org_id order by started_at) as prev_mrr_cents,
        lag(status) over (partition by org_id order by started_at) as prev_status
    from {{ ref('stg_subscriptions') }}
)

select
    sub_id,
    org_id,
    plan,
    status,
    started_at,
    started_date,
    ended_at,
    mrr_cents,
    canceled_reason,
    -- MRR movement classification
    case
        when prev_plan is null and status != 'canceled'
            then 'new'
        when prev_status = 'canceled' and status = 'active'
            then 'reactivation'
        when mrr_cents > coalesce(prev_mrr_cents, 0) and prev_plan is not null
            then 'expansion'
        when mrr_cents < coalesce(prev_mrr_cents, 0) and prev_plan is not null
            then 'contraction'
        when status = 'canceled'
            then 'churn'
        else 'unchanged'
    end                                     as mrr_movement,
    mrr_cents - coalesce(prev_mrr_cents, 0) as mrr_delta_cents
from current_subs
