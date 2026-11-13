{{
  config(
    materialized = 'table',
    description  = 'Experiment exposure fact table with outcomes. '
                   'Randomisation unit: org (not user) to avoid contamination.'
  )
}}

/*
  Experiment exposure table joins PostHog feature-flag assignments to outcomes.
  Assignment is logged server-side when the feature flag is evaluated for an org,
  ensuring the exposure event fires only when the variant is actually seen.

  Production source: PostHog warehouse export → raw_experiment_exposures
  Dev / CI source:   dbt seed (seed_experiment_exposures)
*/

with exposures as (
    select
        experiment_id,
        org_id,
        variant,
        cast(exposed_at as timestamp) as exposed_at,
        cast(exposed_at as date)      as exposed_date
    from {{ ref('seed_experiment_exposures') }}
),

/*  Activation outcome: org ran a flight check within 7 days of exposure  */
activation_outcomes as (
    select
        fc.org_id,
        min(fc.occurred_at) as first_check_after
    from {{ ref('fct_flight_checks') }} fc
    group by fc.org_id
),

/*  Org-level covariates for CUPED variance reduction  */
pre_period_covariates as (
    select
        e.org_id,
        count(fc.check_id) as pre_checks
    from exposures e
    left join {{ ref('fct_flight_checks') }} fc
           on fc.org_id = e.org_id
          and fc.occurred_at < e.exposed_at
    group by e.org_id
)

select
    e.experiment_id,
    e.org_id,
    e.variant,
    e.exposed_at,
    e.exposed_date,

    /*  Primary metric: activated in 7 days (ran ≥1 flight check)  */
    case
        when ao.first_check_after is not null
         and ao.first_check_after >= e.exposed_at
         and ao.first_check_after <= e.exposed_at + interval '7 days'
        then true
        else false
    end as activated_7d,

    /*  Secondary metric: activated within 14 days  */
    case
        when ao.first_check_after is not null
         and ao.first_check_after >= e.exposed_at
         and ao.first_check_after <= e.exposed_at + interval '14 days'
        then true
        else false
    end as activated_14d,

    /*  CUPED covariate: flight checks before exposure  */
    coalesce(pp.pre_checks, 0) as pre_period_checks,

    /*  Org dimension enrichment  */
    o.plan,
    o.signup_channel,
    o.first_framework

from exposures e
left join activation_outcomes  ao using (org_id)
left join pre_period_covariates pp using (org_id)
left join {{ ref('dim_orgs') }}   o  using (org_id)
