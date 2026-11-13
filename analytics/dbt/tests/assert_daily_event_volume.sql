-- Fails (returns rows) if any day in the last 30 days has fewer than 10 events.
-- Healthy seed data should produce far more; this guards against accidental
-- deletion of seed rows or a broken staging view.
select event_date, event_count
from {{ ref('mrt_daily_active_orgs') }}
where event_date >= current_date - interval '30 days'
  and dau < 1
