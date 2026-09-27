package version

import (
	"strings"
	"testing"
)

func TestStringIncludesAllMetadata(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = "9.9.9-test"
	got := String()
	for _, want := range []string{"datadeck", "9.9.9-test", "commit", "built"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
}
