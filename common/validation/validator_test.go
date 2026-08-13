package validation

import (
	"net/http"
	"testing"
)

func TestValidatorDecimalRules(t *testing.T) {
	type request struct {
		Amount string `json:"amount" validate:"required,decimal_gt_zero,decimal_30_8"`
	}
	validator := New()
	if err := validator.Validate(&http.Request{}, &request{Amount: "12.123"}); err != nil {
		t.Fatalf("valid amount rejected: %v", err)
	}
	for _, amount := range []string{"", "0", "-1", "1e3", "1.123456789", "10000000000000000000000"} {
		if err := validator.Validate(&http.Request{}, &request{Amount: amount}); err == nil {
			t.Fatalf("invalid amount %q accepted", amount)
		}
	}
}

func TestValidatorRequired(t *testing.T) {
	type request struct {
		ID int64 `json:"id" validate:"required"`
	}
	if err := New().Validate(&http.Request{}, &request{}); err == nil {
		t.Fatal("missing required field accepted")
	}
}

func TestValidatorDynamicDecimalPrecisionRule(t *testing.T) {
	type request struct {
		Amount string `json:"amount" validate:"decimal_12_3"`
	}
	validator := New()
	if err := validator.Validate(&http.Request{}, &request{Amount: "123456789.123"}); err != nil {
		t.Fatalf("valid dynamic precision amount rejected: %v", err)
	}
	for _, amount := range []string{"1234567890.123", "1.1234"} {
		if err := validator.Validate(&http.Request{}, &request{Amount: amount}); err == nil {
			t.Fatalf("invalid dynamic precision amount %q accepted", amount)
		}
	}
}
