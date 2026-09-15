package cmd

import "testing"

func TestParseRemote(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantHost string
		wantPort string
		wantErr  bool
	}{
		{name: "bare port targets the instance itself", in: "8080", wantPort: "8080"},
		{name: "host and port", in: "db.internal:5432", wantHost: "db.internal", wantPort: "5432"},
		{name: "rds endpoint", in: "my-db.cluster-abc.eu-west-1.rds.amazonaws.com:5432",
			wantHost: "my-db.cluster-abc.eu-west-1.rds.amazonaws.com", wantPort: "5432"},
		{name: "ipv4 literal", in: "10.0.1.5:6379", wantHost: "10.0.1.5", wantPort: "6379"},
		{name: "bracketed ipv6", in: "[fd00::1]:5432", wantHost: "fd00::1", wantPort: "5432"},
		{name: "surrounding space is tolerated", in: "  8080  ", wantPort: "8080"},
		{name: "lowest valid port", in: "1", wantPort: "1"},
		{name: "highest valid port", in: "65535", wantPort: "65535"},

		{name: "empty", in: "", wantErr: true},
		{name: "port zero", in: "0", wantErr: true},
		{name: "port above range", in: "65536", wantErr: true},
		{name: "negative port", in: "-1", wantErr: true},
		{name: "non-numeric port", in: "http", wantErr: true},
		{name: "host with non-numeric port", in: "db.internal:pgsql", wantErr: true},
		{name: "missing host before colon", in: ":5432", wantErr: true},
		{name: "unbracketed ipv6 is ambiguous", in: "fd00::1", wantErr: true},
		{name: "bracketed without port", in: "[fd00::1]", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := parseRemote(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseRemote(%q) = (%q, %q, nil), want error", tc.in, host, port)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRemote(%q) unexpected error: %v", tc.in, err)
			}
			if host != tc.wantHost || port != tc.wantPort {
				t.Errorf("parseRemote(%q) = (%q, %q), want (%q, %q)", tc.in, host, port, tc.wantHost, tc.wantPort)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	for _, ok := range []string{"1", "22", "5432", "65535"} {
		if err := validatePort(ok); err != nil {
			t.Errorf("validatePort(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "0", "65536", "99999", "-5", "80.5", "eighty"} {
		if err := validatePort(bad); err == nil {
			t.Errorf("validatePort(%q) = nil, want error", bad)
		}
	}
}
