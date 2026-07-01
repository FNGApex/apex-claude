package version

import (
	"regexp"
	"testing"
)

func TestVersionFormat(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) {
		t.Errorf("Version %q does not match MAJOR.MINOR.PATCH", Version)
	}
}
