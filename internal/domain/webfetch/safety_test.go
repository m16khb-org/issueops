package webfetch

import "testing"

func TestFetchURLPolicyKeepsReservedNetworksBlocked(t *testing.T) {
	for _, raw := range []string{"http://169.254.169.254/latest/meta-data", "http://100.64.1.1/", "http://192.0.2.1/", "http://198.18.1.1/", "http://198.51.100.2/", "http://203.0.113.9/", "http://240.0.0.1/", "http://[2001:db8::1]/", "http://[::ffff:169.254.169.254]/"} {
		for _, allow := range []bool{false, true} {
			if _, err := ParseFetchURL(raw, allow); err == nil {
				t.Errorf("reserved URL accepted with allowPrivate=%v: %s", allow, raw)
			}
		}
	}
	for _, raw := range []string{"http://localhost/", "http://127.0.0.1/", "http://10.0.0.1/", "http://[::1]/"} {
		if _, err := ParseFetchURL(raw, false); err == nil {
			t.Errorf("private URL accepted: %s", raw)
		}
		if _, err := ParseFetchURL(raw, true); err != nil {
			t.Errorf("private fixture rejected: %s: %v", raw, err)
		}
	}
	for _, raw := range []string{"file:///tmp/source", "https://user:password@example.invalid", "https:///no-host"} {
		if _, err := ParseFetchURL(raw, true); err == nil {
			t.Errorf("invalid URL accepted: %s", raw)
		}
	}
}
