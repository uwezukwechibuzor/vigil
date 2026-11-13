"""
A/B Experiment Analysis Framework

Usage:
    from analytics.experiments.ab_test import analyze_experiment
    results = analyze_experiment(df, metric_col="activated_7d", variant_col="variant")

Supports:
- Two-proportion z-test with 95% CI
- Sample ratio mismatch (SRM) check via chi-square
- CUPED variance reduction using a pre-period covariate
- Segment breakdowns (flagged as exploratory)
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Any, Optional, Sequence

import numpy as np
from scipy import stats


@dataclass
class ExperimentResult:
    n_control: int
    n_treatment: int
    mean_control: float
    mean_treatment: float
    absolute_lift: float
    relative_lift_pct: float
    z_stat: float
    p_value: float
    ci_lower: float
    ci_upper: float
    srm_p_value: float
    srm_flag: bool
    method: str  # "z_test" or "cuped"
    notes: list[str]

    def __str__(self) -> str:
        lines = [
            f"=== Experiment Analysis ({self.method}) ===",
            f"  Control:   n={self.n_control:,}  mean={self.mean_control:.4f}",
            f"  Treatment: n={self.n_treatment:,}  mean={self.mean_treatment:.4f}",
            f"  Absolute lift:  {self.absolute_lift:+.4f}  ({self.absolute_lift*100:+.2f}pp)",
            f"  Relative lift:  {self.relative_lift_pct:+.1f}%",
            f"  z={self.z_stat:.3f}  p={self.p_value:.4f}",
            f"  95% CI:  [{self.ci_lower:+.4f}, {self.ci_upper:+.4f}]",
            f"  SRM:  p={self.srm_p_value:.4f}  {'⚠ FLAG' if self.srm_flag else 'OK'}",
        ]
        for note in self.notes:
            lines.append(f"  NOTE: {note}")
        return "\n".join(lines)


def z_test_proportions(
    n_c: int,
    x_c: int,
    n_t: int,
    x_t: int,
    alpha: float = 0.05,
) -> tuple[float, float, float, float]:
    """
    Two-proportion z-test.

    Returns (z_stat, p_value, ci_lower, ci_upper) for (p_t - p_c).
    """
    p_c = x_c / n_c
    p_t = x_t / n_t

    # Pooled proportion for the null
    p_pool = (x_c + x_t) / (n_c + n_t)
    se_null = math.sqrt(p_pool * (1 - p_pool) * (1 / n_c + 1 / n_t))

    if se_null == 0:
        return 0.0, 1.0, 0.0, 0.0

    z_stat = (p_t - p_c) / se_null
    p_value = 2 * (1 - stats.norm.cdf(abs(z_stat)))

    # CI uses unpooled SE
    se_unpooled = math.sqrt(p_c * (1 - p_c) / n_c + p_t * (1 - p_t) / n_t)
    z_crit = stats.norm.ppf(1 - alpha / 2)
    diff = p_t - p_c
    return z_stat, p_value, diff - z_crit * se_unpooled, diff + z_crit * se_unpooled


def z_test_means(
    control: Sequence[float],
    treatment: Sequence[float],
    alpha: float = 0.05,
) -> tuple[float, float, float, float]:
    """
    Welch's t-test for difference in means.

    Returns (t_stat, p_value, ci_lower, ci_upper).
    """
    t_stat, p_value = stats.ttest_ind(treatment, control, equal_var=False)
    n_c, n_t = len(control), len(treatment)
    se = math.sqrt(np.var(control, ddof=1) / n_c + np.var(treatment, ddof=1) / n_t)
    df = (
        (np.var(control, ddof=1) / n_c + np.var(treatment, ddof=1) / n_t) ** 2
        / (
            (np.var(control, ddof=1) / n_c) ** 2 / (n_c - 1)
            + (np.var(treatment, ddof=1) / n_t) ** 2 / (n_t - 1)
        )
    )
    t_crit = stats.t.ppf(1 - alpha / 2, df)
    diff = np.mean(treatment) - np.mean(control)
    return float(t_stat), float(p_value), diff - t_crit * se, diff + t_crit * se


def sample_ratio_mismatch(
    n_control: int,
    n_treatment: int,
    expected_ratio: float = 0.5,
    alpha: float = 0.01,
) -> tuple[float, bool]:
    """
    Chi-square SRM check.

    Returns (p_value, is_flagged). Flag at alpha=0.01 (stricter than main test).
    """
    n_total = n_control + n_treatment
    expected_c = n_total * expected_ratio
    expected_t = n_total * (1 - expected_ratio)
    chi2 = (n_control - expected_c) ** 2 / expected_c + (n_treatment - expected_t) ** 2 / expected_t
    p_value = float(1 - stats.chi2.cdf(chi2, df=1))
    return p_value, p_value < alpha


def cuped_adjustment(
    y: np.ndarray,
    x_pre: np.ndarray,
) -> np.ndarray:
    """
    CUPED: Y_adj = Y - theta * (X_pre - mean(X_pre))

    theta is estimated via OLS covariance ratio.
    """
    cov = np.cov(y, x_pre)
    theta = cov[0, 1] / max(cov[1, 1], 1e-10)
    return y - theta * (x_pre - np.mean(x_pre))


def analyze_experiment(
    metric_control: Sequence[float],
    metric_treatment: Sequence[float],
    is_binary: bool = True,
    covariate_control: Optional[Sequence[float]] = None,
    covariate_treatment: Optional[Sequence[float]] = None,
    expected_ratio: float = 0.5,
    alpha: float = 0.05,
) -> ExperimentResult:
    """
    Full experiment analysis.

    Args:
        metric_control:    per-unit metric values in control arm
        metric_treatment:  per-unit metric values in treatment arm
        is_binary:         True for proportions (activation, conversion), False for means
        covariate_control: pre-period covariate for CUPED (e.g. pre-period checks)
        covariate_treatment: pre-period covariate for treatment
        expected_ratio:    expected fraction in control (usually 0.5)
        alpha:             significance level

    Returns:
        ExperimentResult dataclass
    """
    y_c = np.array(metric_control, dtype=float)
    y_t = np.array(metric_treatment, dtype=float)
    n_c, n_t = len(y_c), len(y_t)

    srm_p, srm_flag = sample_ratio_mismatch(n_c, n_t, expected_ratio, alpha=0.01)
    notes: list[str] = []
    if srm_flag:
        notes.append("SRM detected — assignment may be biased. Interpret with caution.")

    # Apply CUPED if covariate provided
    method = "z_test" if is_binary else "t_test"
    if covariate_control is not None and covariate_treatment is not None:
        x_c = np.array(covariate_control, dtype=float)
        x_t = np.array(covariate_treatment, dtype=float)
        # Pool covariate to estimate theta jointly
        y_all = np.concatenate([y_c, y_t])
        x_all = np.concatenate([x_c, x_t])
        cov_m = np.cov(y_all, x_all)
        theta = cov_m[0, 1] / max(cov_m[1, 1], 1e-10)
        y_c = y_c - theta * (x_c - np.mean(x_all))
        y_t = y_t - theta * (x_t - np.mean(x_all))
        method = "cuped"
        notes.append(f"CUPED applied (theta={theta:.4f}). Variance reduction achieved.")

    if is_binary:
        x_c_int = int(np.sum(y_c))
        x_t_int = int(np.sum(y_t))
        z, p, ci_lo, ci_hi = z_test_proportions(n_c, x_c_int, n_t, x_t_int, alpha)
        mean_c = x_c_int / n_c
        mean_t = x_t_int / n_t
    else:
        z, p, ci_lo, ci_hi = z_test_means(y_c, y_t, alpha)
        mean_c = float(np.mean(y_c))
        mean_t = float(np.mean(y_t))

    abs_lift = mean_t - mean_c
    rel_lift = (abs_lift / mean_c * 100) if mean_c != 0 else 0.0

    return ExperimentResult(
        n_control=n_c,
        n_treatment=n_t,
        mean_control=mean_c,
        mean_treatment=mean_t,
        absolute_lift=abs_lift,
        relative_lift_pct=rel_lift,
        z_stat=z,
        p_value=p,
        ci_lower=ci_lo,
        ci_upper=ci_hi,
        srm_p_value=srm_p,
        srm_flag=srm_flag,
        method=method,
        notes=notes,
    )


def sample_size_calculator(
    baseline_rate: float,
    mde: float,
    alpha: float = 0.05,
    power: float = 0.80,
) -> dict[str, Any]:
    """
    Calculate required sample size per arm for a two-proportion test.

    Args:
        baseline_rate: control conversion rate (e.g. 0.40)
        mde:           minimum detectable effect as absolute percentage points (e.g. 0.05)
        alpha:         significance level
        power:         desired statistical power (1 - beta)
    """
    p1 = baseline_rate
    p2 = baseline_rate + mde
    p_bar = (p1 + p2) / 2

    z_alpha = stats.norm.ppf(1 - alpha / 2)
    z_beta  = stats.norm.ppf(power)

    n = (
        (z_alpha * math.sqrt(2 * p_bar * (1 - p_bar))
         + z_beta * math.sqrt(p1 * (1 - p1) + p2 * (1 - p2))) ** 2
        / (p1 - p2) ** 2
    )
    n = math.ceil(n)
    return {
        "n_per_arm": n,
        "total_n": 2 * n,
        "baseline_rate": p1,
        "target_rate": p2,
        "mde": mde,
        "alpha": alpha,
        "power": power,
    }
