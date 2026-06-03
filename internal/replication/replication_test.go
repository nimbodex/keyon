package replication

import "testing"

func TestParseResponse(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"ok empty", respOK, "", false},
		{"ok payload", respOK + " wal_1.log wal_2.log", "wal_1.log wal_2.log", false},
		{"error", respError + " boom", "", true},
		{"unexpected", "garbage", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseResponse(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
