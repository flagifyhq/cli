package codegen

import (
	"go/format"
	"strings"
	"testing"
)

func TestGenerateGo_IsGofmtIdempotent(t *testing.T) {
	cases := map[string][]string{
		"multi":  {"dark-mode", "enable-ai-features", "new-checkout-flow", "onboarding-v2"},
		"single": {"dark-mode"},
		"empty":  {},
	}
	for name, keys := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := GenerateGo(flagsFromKeys(keys...), "Demo Project", "myflags")
			if err != nil {
				t.Fatalf("GenerateGo: %v", err)
			}
			formatted, err := format.Source([]byte(out))
			if err != nil {
				t.Fatalf("gofmt failed on generated source: %v\n%s", err, out)
			}
			if string(formatted) != out {
				t.Errorf("generated Go is not gofmt-idempotent.\n--- generated ---\n%s\n--- gofmt ---\n%s\n--- diff hint ---\n%s",
					out, string(formatted), stringDiff(out, string(formatted)))
			}
		})
	}
}

func stringDiff(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	var diff strings.Builder
	max := len(la)
	if len(lb) > max {
		max = len(lb)
	}
	for i := 0; i < max; i++ {
		var lineA, lineB string
		if i < len(la) {
			lineA = la[i]
		}
		if i < len(lb) {
			lineB = lb[i]
		}
		if lineA != lineB {
			diff.WriteString("- " + lineA + "\n")
			diff.WriteString("+ " + lineB + "\n")
		}
	}
	return diff.String()
}
