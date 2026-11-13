package check_test

import (
	"context"
	"testing"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/check"
)

func ptr[T any](v T) *T { return &v }

// deps with no external connections — most checks degrade gracefully.
var nilDeps = &check.Deps{}

// ----------------------------------------------------------------------------
// infra_health
// ----------------------------------------------------------------------------

func TestInfraHealth_NoClient(t *testing.T) {
	c := &check.InfraHealthCheck{}
	req := &check.Request{ChainID: 1}
	res, err := c.Run(context.Background(), req, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusFail {
		t.Errorf("expected fail, got %s", res.Status)
	}
}

// ----------------------------------------------------------------------------
// oracle_freshness
// ----------------------------------------------------------------------------

func TestOracleFreshness_NoDeadline(t *testing.T) {
	c := &check.OracleFreshnessCheck{}
	res, err := c.Run(context.Background(), &check.Request{}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusPass {
		t.Errorf("expected pass, got %s", res.Status)
	}
}

func TestOracleFreshness_ExpiredDeadline(t *testing.T) {
	c := &check.OracleFreshnessCheck{}
	past := time.Now().Add(-10 * time.Minute).Unix()
	res, err := c.Run(context.Background(), &check.Request{DeadlineS: &past}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusFail {
		t.Errorf("expected fail, got %s", res.Status)
	}
}

func TestOracleFreshness_ValidDeadline(t *testing.T) {
	c := &check.OracleFreshnessCheck{}
	future := time.Now().Add(10 * time.Minute).Unix()
	res, err := c.Run(context.Background(), &check.Request{DeadlineS: &future}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusPass {
		t.Errorf("expected pass, got %s", res.Status)
	}
}

// ----------------------------------------------------------------------------
// slippage
// ----------------------------------------------------------------------------

func TestSlippage_NotApplicable(t *testing.T) {
	c := &check.SlippageCheck{}
	res, err := c.Run(context.Background(), &check.Request{MaxSlippageBPS: 0}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusPass {
		t.Errorf("expected pass, got %s", res.Status)
	}
}

func TestSlippage_ExpiredDeadline(t *testing.T) {
	c := &check.SlippageCheck{}
	past := time.Now().Add(-1 * time.Minute).Unix()
	res, err := c.Run(context.Background(), &check.Request{
		MaxSlippageBPS: 50,
		ExpectedOut:    ptr("0x64"),
		DeadlineS:      &past,
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusFail {
		t.Errorf("expected fail, got %s", res.Status)
	}
}

func TestSlippage_WithinTolerance(t *testing.T) {
	c := &check.SlippageCheck{}
	// value = 100 hex (0x64), expected = 0x64 → 0 bps slippage
	res, err := c.Run(context.Background(), &check.Request{
		MaxSlippageBPS: 50,
		Value:          "0x64", // 100
		ExpectedOut:    ptr("0x64"),
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusPass {
		t.Errorf("expected pass, got %s: %s", res.Status, res.Reason)
	}
}

func TestSlippage_ExceedsTolerance(t *testing.T) {
	c := &check.SlippageCheck{}
	// value = 90, expected = 100 → 1000 bps slippage
	res, err := c.Run(context.Background(), &check.Request{
		MaxSlippageBPS: 50,
		Value:          "0x5a", // 90
		ExpectedOut:    ptr("100"),
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusFail {
		t.Errorf("expected fail, got %s: %s", res.Status, res.Reason)
	}
}

// ----------------------------------------------------------------------------
// gas_sanity
// ----------------------------------------------------------------------------

func TestGasSanity_NoClient(t *testing.T) {
	c := &check.GasSanityCheck{}
	res, err := c.Run(context.Background(), &check.Request{}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusWarn {
		t.Errorf("expected warn without client, got %s", res.Status)
	}
}

func TestGasSanity_HighGasLimit(t *testing.T) {
	// Without a live client we can only check the gas param branch.
	// Since gas is provided, the check warns but cannot call SuggestGasPrice.
	// We test with nilDeps (no client) and a high gas limit.
	c := &check.GasSanityCheck{}
	gas := uint64(2_000_000)
	res, err := c.Run(context.Background(), &check.Request{Gas: &gas}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	// No client → always StatusWarn regardless of gas param
	if res.Status != check.StatusWarn {
		t.Errorf("expected warn, got %s", res.Status)
	}
}

// ----------------------------------------------------------------------------
// contract_risk
// ----------------------------------------------------------------------------

func TestContractRisk_NoDataNoClient(t *testing.T) {
	c := &check.ContractRiskCheck{}
	res, err := c.Run(context.Background(), &check.Request{
		To:   "0x000000000000000000000000000000000000dEaD",
		Data: "0x",
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	// Without client, no code check; no data = no selector. Should pass.
	if res.Status != check.StatusPass {
		t.Errorf("expected pass, got %s: %s", res.Status, res.Reason)
	}
}

func TestContractRisk_UnlimitedApproval(t *testing.T) {
	c := &check.ContractRiskCheck{}
	// approve(address, 2^256-1)
	// 095ea7b3 + 32-byte address + 32-byte max uint256
	maxU256 := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	spender := "000000000000000000000000ffffffffffffffffffffffffffffffffffffffff"
	data := "0x095ea7b3" + spender + maxU256
	res, err := c.Run(context.Background(), &check.Request{
		To:   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		Data: data,
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusWarn {
		t.Errorf("expected warn for unlimited approval, got %s: %s", res.Status, res.Reason)
	}
}

// ----------------------------------------------------------------------------
// policy
// ----------------------------------------------------------------------------

func TestPolicy_NoPolicyID(t *testing.T) {
	c := &check.PolicyCheck{}
	res, err := c.Run(context.Background(), &check.Request{}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusPass {
		t.Errorf("expected pass with no policy, got %s", res.Status)
	}
}

func TestPolicy_InvalidPolicyIDFormat(t *testing.T) {
	c := &check.PolicyCheck{}
	res, err := c.Run(context.Background(), &check.Request{PolicyID: ptr("not-a-uuid")}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusFail {
		t.Errorf("expected fail for invalid uuid, got %s", res.Status)
	}
}

func TestPolicy_NoDB(t *testing.T) {
	c := &check.PolicyCheck{}
	policyID := "00000000-0000-0000-0000-000000000001"
	res, err := c.Run(context.Background(), &check.Request{PolicyID: &policyID}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusWarn {
		t.Errorf("expected warn without db, got %s", res.Status)
	}
}

// ----------------------------------------------------------------------------
// address_safety
// ----------------------------------------------------------------------------

func TestAddressSafety_NilScreenerNoDB(t *testing.T) {
	c := &check.AddressSafetyCheck{Screener: nil}
	res, err := c.Run(context.Background(), &check.Request{
		To:        "0x1234567890123456789012345678901234567890",
		ProjectID: "",
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusPass {
		t.Errorf("expected pass, got %s", res.Status)
	}
}

// mockScreener is a test-only screener.
type mockScreener struct {
	level  string
	reason string
}

func (m *mockScreener) Screen(_ context.Context, _ string) (string, string, error) {
	return m.level, m.reason, nil
}

func TestAddressSafety_ScreenerHigh(t *testing.T) {
	c := &check.AddressSafetyCheck{Screener: &mockScreener{level: "high", reason: "ofac sanction"}}
	res, err := c.Run(context.Background(), &check.Request{
		To: "0x1234567890123456789012345678901234567890",
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusFail {
		t.Errorf("expected fail for high risk, got %s", res.Status)
	}
}

func TestAddressSafety_ScreenerMedium(t *testing.T) {
	c := &check.AddressSafetyCheck{Screener: &mockScreener{level: "medium", reason: "mixer"}}
	res, err := c.Run(context.Background(), &check.Request{
		To: "0x1234567890123456789012345678901234567890",
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusWarn {
		t.Errorf("expected warn for medium risk, got %s", res.Status)
	}
}

func TestAddressSafety_ScreenerNone(t *testing.T) {
	c := &check.AddressSafetyCheck{Screener: &mockScreener{level: "none", reason: ""}}
	res, err := c.Run(context.Background(), &check.Request{
		To: "0x1234567890123456789012345678901234567890",
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusPass {
		t.Errorf("expected pass for none risk, got %s", res.Status)
	}
}

// ----------------------------------------------------------------------------
// simulation (no live client)
// ----------------------------------------------------------------------------

func TestSimulation_NoClient(t *testing.T) {
	c := &check.SimulationCheck{}
	res, err := c.Run(context.Background(), &check.Request{
		From:  "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		To:    "0x000000000000000000000000000000000000dEaD",
		Value: "0x0",
		Data:  "0x",
	}, nilDeps)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusWarn {
		t.Errorf("expected warn without client, got %s", res.Status)
	}
}
