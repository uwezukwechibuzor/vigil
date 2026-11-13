{#
  activation_macro v1.0 — 2026-11-13
  Definition: an org is "activated" if it ran at least one real (non-test)
  flight check within {{ var('activation_days', 7) }} days of signing up.

  This macro is versioned; bump the version comment when the definition changes
  so historical metrics can be recomputed consistently.

  Usage:
    {{ is_activated('signup_date_col', 'first_flight_check_date_col') }}
  Returns: boolean expression
#}
{% macro is_activated(signup_date, first_check_date) %}
    {{ first_check_date }} is not null
    and datediff('day', cast({{ signup_date }} as date), cast({{ first_check_date }} as date))
        <= {{ var('activation_days', 7) }}
{% endmacro %}


{#
  date_spine helper: generate a series of dates between start and end.
  Used in retention and DAU mart models.
#}
{% macro date_spine(start_date, end_date) %}
    generate_series(
        cast('{{ start_date }}' as date),
        cast('{{ end_date }}' as date),
        interval '1 day'
    )
{% endmacro %}
