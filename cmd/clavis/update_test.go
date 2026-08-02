package main

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name    string
		current string
		tag     string
		want    int
	}{
		{"older patch", "0.1.0", "v0.1.1", cmpOlder},
		{"older minor", "0.1.0", "v0.2.0", cmpOlder},
		{"older major", "0.9.9", "v1.0.0", cmpOlder},
		{"equal", "0.1.0", "v0.1.0", cmpEqual},
		{"equal without v on tag", "0.1.0", "0.1.0", cmpEqual},
		{"newer than release", "0.9.9", "v0.1.0", cmpNewer},
		{"newer patch", "1.2.4", "v1.2.3", cmpNewer},
		{"dev build", "dev", "v0.1.0", cmpDev},
		{"empty current", "", "v0.1.0", cmpDev},
		{"git sha current", "5ddc0db-dirty", "v0.1.0", cmpDev},
		{"bad tag", "0.1.0", "not-a-version", cmpDev},
		// semver niceties
		{"prerelease older than release", "1.0.0-rc1", "v1.0.0", cmpOlder},
		{"current has leading v", "v0.1.0", "v0.2.0", cmpOlder},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compareVersions(c.current, c.tag); got != c.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", c.current, c.tag, got, c.want)
			}
		})
	}
}
