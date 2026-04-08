package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToKebabCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"devMode", "dev-mode"},
		{"dev_mode", "dev-mode"},
		{"DevMode", "dev-mode"},
		{"dev mode", "dev-mode"},
		{"newCheckoutFlow", "new-checkout-flow"},
		{"new_checkout_flow", "new-checkout-flow"},
		{"NewCheckoutFlow", "new-checkout-flow"},
		{"already-kebab", "already-kebab"},
		{"simple", "simple"},
		{"ABCDef", "abcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, toKebabCase(tt.input))
		})
	}
}

func TestKebabCaseValidation(t *testing.T) {
	valid := []string{
		"dev-mode",
		"new-checkout-flow",
		"simple",
		"a1-b2-c3",
	}
	for _, key := range valid {
		t.Run("valid:"+key, func(t *testing.T) {
			assert.True(t, kebabCaseRe.MatchString(key), "expected %q to be valid kebab-case", key)
		})
	}

	invalid := []string{
		"devMode",
		"dev_mode",
		"DevMode",
		"dev mode",
		"-leading-dash",
		"trailing-dash-",
		"UPPERCASE",
		"123-starts-with-number",
		"",
	}
	for _, key := range invalid {
		t.Run("invalid:"+key, func(t *testing.T) {
			assert.False(t, kebabCaseRe.MatchString(key), "expected %q to be invalid kebab-case", key)
		})
	}
}
