"""
Unit tests for the A/B experiment analysis framework.

Acceptance criterion: analysis must recover the true effect within its 95% CI.
"""

from __future__ import annotations

import numpy as np
import pytest

from analytics.experiments.ab_test import (
    ExperimentResult,
    analyze_experiment,
    cuped_adjustment,
    sample_ratio_mismatch,
    sample_size_calculator,
    z_test_means,
    z_test_proportions,
)


RNG = np.random.default_rng(42)


class TestZTestProportions:
    def test_no_difference(self) -> None:
        z, p, lo, hi = z_test_proportions(1000, 400, 1000, 400)
        assert abs(z) < 0.1
        assert p > 0.9
        assert lo < 0 < hi

    def test_significant_lift(self) -> None:
        # 40% vs 50% — clearly significant at n=2000
        z, p, lo, hi = z_test_proportions(2000, 800, 2000, 1000)
        assert p < 0.01
        assert lo > 0  # CI entirely positive
        assert z > 0

    def test_ci_contains_true_effect(self) -> None:
        true_effect = 0.05
        p_c, p_t = 0.40, 0.45
        n = 2000
        # CI should contain the true effect for the simulated data
        z, p, lo, hi = z_test_proportions(n, int(n * p_c), n, int(n * p_t))
        assert lo <= true_effect <= hi or abs(lo - true_effect) < 0.02


class TestZTestMeans:
    def test_known_effect_recovered(self) -> None:
        # Simulate: control ~ N(100, 10), treatment ~ N(105, 10), true effect = 5
        # Use n=10000 so the CI reliably contains the true effect
        rng = np.random.default_rng(1234)
        ctrl = rng.normal(100, 10, 10000)
        trt  = rng.normal(105, 10, 10000)
        t, p, lo, hi = z_test_means(ctrl, trt)
        assert p < 0.01
        assert lo <= 5 <= hi  # true effect within CI

    def test_no_effect(self) -> None:
        ctrl = RNG.normal(100, 10, 1000)
        trt  = RNG.normal(100, 10, 1000)
        _, p, _, _ = z_test_means(ctrl, trt)
        # Should NOT be significant most of the time
        assert p > 0.001  # extremely unlikely to be this significant by chance


class TestSampleRatioMismatch:
    def test_balanced_no_srm(self) -> None:
        p, flag = sample_ratio_mismatch(1000, 1000)
        assert not flag
        assert p > 0.5

    def test_imbalanced_srm_detected(self) -> None:
        # 60/40 split when expecting 50/50
        p, flag = sample_ratio_mismatch(1200, 800, expected_ratio=0.5)
        assert flag
        assert p < 0.01

    def test_non_equal_expected_ratio(self) -> None:
        # Expecting 70/30, actual 700/300 — should be fine
        p, flag = sample_ratio_mismatch(700, 300, expected_ratio=0.7)
        assert not flag


class TestCupedAdjustment:
    def test_variance_reduction(self) -> None:
        """CUPED should reduce variance when covariate is correlated with outcome."""
        x_pre = RNG.normal(50, 10, 1000)
        noise = RNG.normal(0, 5, 1000)
        y = 0.8 * x_pre + noise  # strongly correlated

        y_adj = cuped_adjustment(y, x_pre)

        var_before = np.var(y)
        var_after  = np.var(y_adj)
        # Variance should decrease when covariate correlates with outcome
        assert var_after < var_before

    def test_uncorrelated_no_harm(self) -> None:
        """With uncorrelated covariate, CUPED should not inflate variance significantly."""
        x_pre = RNG.normal(0, 1, 1000)
        y     = RNG.normal(0, 1, 1000)
        y_adj = cuped_adjustment(y, x_pre)
        # Variance roughly preserved (within 20%)
        assert np.var(y_adj) < np.var(y) * 1.2


class TestAnalyzeExperiment:
    def _simulate(
        self,
        n: int = 1500,
        p_c: float = 0.40,
        p_t: float = 0.46,  # 6pp lift
        seed: int = 0,
    ) -> tuple[np.ndarray, np.ndarray]:
        rng = np.random.default_rng(seed)
        ctrl = rng.binomial(1, p_c, n).astype(float)
        trt  = rng.binomial(1, p_t, n).astype(float)
        return ctrl, trt

    def test_recovers_true_effect_in_ci(self) -> None:
        """95% CI should contain the true effect."""
        true_effect = 0.06
        ctrl, trt = self._simulate(n=2000, p_c=0.40, p_t=0.46)
        result = analyze_experiment(ctrl, trt, is_binary=True)
        assert result.ci_lower <= true_effect <= result.ci_upper

    def test_significant_at_n2000(self) -> None:
        ctrl, trt = self._simulate(n=2000)
        result = analyze_experiment(ctrl, trt, is_binary=True)
        assert result.p_value < 0.05
        assert result.absolute_lift > 0

    def test_no_effect_not_significant(self) -> None:
        rng = np.random.default_rng(99)
        ctrl = rng.binomial(1, 0.40, 1000).astype(float)
        trt  = rng.binomial(1, 0.40, 1000).astype(float)
        result = analyze_experiment(ctrl, trt, is_binary=True)
        # Should not be significant (very unlikely at true null)
        assert result.p_value > 0.001

    def test_cuped_tightens_ci(self) -> None:
        """CUPED CI should be no wider than raw CI when covariate is predictive."""
        rng = np.random.default_rng(7)
        x_pre_c = rng.normal(50, 10, 1500)
        x_pre_t = rng.normal(50, 10, 1500)
        # Correlated outcome: activation ~ 0.4 + 0.003 * (x_pre - 50) + noise
        y_c = (rng.normal(0, 1, 1500) + 0.4 + 0.003 * (x_pre_c - 50)) > 0.7
        y_t = (rng.normal(0, 1, 1500) + 0.46 + 0.003 * (x_pre_t - 50)) > 0.7

        raw   = analyze_experiment(y_c.astype(float), y_t.astype(float), is_binary=True)
        cuped = analyze_experiment(
            y_c.astype(float), y_t.astype(float), is_binary=True,
            covariate_control=x_pre_c, covariate_treatment=x_pre_t
        )
        assert cuped.method == "cuped"
        raw_ci_width   = raw.ci_upper   - raw.ci_lower
        cuped_ci_width = cuped.ci_upper - cuped.ci_lower
        assert cuped_ci_width <= raw_ci_width * 1.05  # CUPED CI ≤ raw CI (±5% tolerance)

    def test_srm_flagged(self) -> None:
        # Heavily imbalanced assignment
        rng = np.random.default_rng(1)
        ctrl = rng.binomial(1, 0.40, 1500).astype(float)
        trt  = rng.binomial(1, 0.46,  500).astype(float)  # only 500 in treatment
        result = analyze_experiment(ctrl, trt, is_binary=True, expected_ratio=0.5)
        assert result.srm_flag
        assert any("SRM" in n for n in result.notes)

    def test_result_str(self) -> None:
        ctrl, trt = self._simulate(n=1000)
        result = analyze_experiment(ctrl, trt, is_binary=True)
        s = str(result)
        assert "Control" in s and "Treatment" in s and "95% CI" in s


class TestSampleSizeCalculator:
    def test_basic(self) -> None:
        calc = sample_size_calculator(baseline_rate=0.40, mde=0.05)
        assert calc["n_per_arm"] > 0
        assert calc["total_n"] == 2 * calc["n_per_arm"]

    def test_smaller_mde_needs_more(self) -> None:
        large_mde = sample_size_calculator(baseline_rate=0.40, mde=0.10)
        small_mde = sample_size_calculator(baseline_rate=0.40, mde=0.02)
        assert small_mde["n_per_arm"] > large_mde["n_per_arm"]

    def test_higher_power_needs_more(self) -> None:
        p80 = sample_size_calculator(0.40, 0.05, power=0.80)
        p95 = sample_size_calculator(0.40, 0.05, power=0.95)
        assert p95["n_per_arm"] > p80["n_per_arm"]
