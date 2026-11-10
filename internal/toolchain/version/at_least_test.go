package version

import "testing"

func TestAtLeast(t *testing.T) {
	cases := []struct {
		have, want string
		wantOK     bool
		wantErr    bool
	}{
		{"0.1", "0.1", true, false},
		{"0.2", "0.1", true, false},
		{"0.1", "0.2", false, false},
		{"0.10", "0.2", true, false},
		{"1", "0.9", true, false},
		{"0.1", "0.1.0", true, false},
		{"0.1", "beta", false, true},
		{"x", "0.1", false, true},
	}
	for _, tc := range cases {
		got, err := AtLeast(tc.have, tc.want)
		if (err != nil) != tc.wantErr {
			t.Errorf("AtLeast(%q, %q) error = %v, wantErr %v", tc.have, tc.want, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.wantOK {
			t.Errorf("AtLeast(%q, %q) = %v, want %v", tc.have, tc.want, got, tc.wantOK)
		}
	}
}
