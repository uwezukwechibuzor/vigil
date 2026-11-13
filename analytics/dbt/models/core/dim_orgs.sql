{{
  config(materialized = 'table')
}}

with orgs as (
    select * from {{ ref('stg_orgs') }}
),

first_checks as (
    select
        org_id,
        min(occurred_at) as first_check_at
    from {{ ref('stg_flight_checks') }}
    group by org_id
),

sub_info as (
    select
        org_id,
        min(started_at)   as first_paid_at,
        max(mrr_cents)    as max_mrr_cents,
        sum(case when status = 'active' then mrr_cents else 0 end) as current_mrr_cents
    from {{ ref('stg_subscriptions') }}
    group by org_id
)

select
    o.org_id,
    o.name,
    o.plan,
    o.signup_date,
    o.created_at                        as signup_at,
    o.signup_channel,
    o.first_framework,
    o.first_chain,
    fc.first_check_at,
    -- Activation: first flight check within N days of signup (versioned macro)
    {{ is_activated('o.created_at', 'fc.first_check_at') }} as is_activated,
    case
        when fc.first_check_at is not null
        then datediff('day', cast(o.created_at as date), cast(fc.first_check_at as date))
    end                                 as days_to_first_value,
    s.first_paid_at,
    s.max_mrr_cents,
    coalesce(s.current_mrr_cents, 0)    as current_mrr_cents,
    s.first_paid_at is not null         as is_paid
from orgs o
left join first_checks fc using (org_id)
left join sub_info s using (org_id)
