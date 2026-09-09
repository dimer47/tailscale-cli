package routes

import (
	"testing"
)

func dev(id, name string, advertised, enabled []string) Device {
	return Device{
		NodeID:           id,
		Name:             name,
		AdvertisedRoutes: advertised,
		EnabledRoutes:    enabled,
		ConnectedToProxy: true,
	}
}

func TestShortName(t *testing.T) {
	tests := []struct {
		name string
		dev  Device
		want string
	}{
		{"strips tailnet suffix", Device{Name: "printstation-013.tailfabc8.ts.net"}, "printstation-013"},
		{"bare name kept", Device{Name: "nas"}, "nas"},
		{"falls back to hostname", Device{Hostname: "AGRIPLPRTST013"}, "AGRIPLPRTST013"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dev.ShortName(); got != tt.want {
				t.Errorf("ShortName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindConflictsNoOverlap(t *testing.T) {
	devices := []Device{
		dev("n1", "a", []string{"192.168.1.0/24"}, []string{"192.168.1.0/24"}),
		dev("n2", "b", []string{"192.168.33.0/24"}, []string{"192.168.33.0/24"}),
	}
	if got := FindConflicts(devices); len(got) != 0 {
		t.Errorf("expected no conflicts, got %d: %+v", len(got), got)
	}
}

func TestFindConflictsExactDuplicate(t *testing.T) {
	devices := []Device{
		dev("n1", "printstation", []string{"192.168.1.0/24"}, []string{"192.168.1.0/24"}),
		dev("n2", "nas", []string{"192.168.1.0/24"}, []string{"192.168.1.0/24"}),
	}
	got := FindConflicts(devices)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %+v", len(got), got)
	}
	if got[0].Prefix != "192.168.1.0/24" {
		t.Errorf("prefix = %q, want 192.168.1.0/24", got[0].Prefix)
	}
	if n := len(got[0].Holders); n != 2 {
		t.Fatalf("expected 2 holders, got %d", n)
	}
	if n := len(got[0].ActiveHolders()); n != 2 {
		t.Errorf("expected 2 active holders, got %d", n)
	}
}

func TestFindConflictsOverlappingPrefixes(t *testing.T) {
	// A /25 sits inside a /24: Tailscale prefers the longer prefix, but both
	// devices are still competing for the same addresses.
	devices := []Device{
		dev("n1", "hangar", []string{"192.168.1.0/25"}, []string{"192.168.1.0/25"}),
		dev("n2", "nas", []string{"192.168.1.0/24"}, []string{"192.168.1.0/24"}),
	}
	if got := FindConflicts(devices); len(got) == 0 {
		t.Error("expected overlap between /25 and /24 to be reported")
	}
}

func TestFindConflictsIgnoresExitNodes(t *testing.T) {
	// Every exit node advertises 0.0.0.0/0; that is expected, not a conflict.
	devices := []Device{
		dev("n1", "a", []string{"0.0.0.0/0", "::/0"}, []string{"0.0.0.0/0", "::/0"}),
		dev("n2", "b", []string{"0.0.0.0/0", "::/0"}, []string{"0.0.0.0/0", "::/0"}),
	}
	if got := FindConflicts(devices); len(got) != 0 {
		t.Errorf("exit node routes must not conflict, got %+v", got)
	}
}

func TestFindConflictsLatent(t *testing.T) {
	// Advertised but not approved: the conflict exists but is not yet live.
	devices := []Device{
		dev("n1", "a", []string{"10.0.0.0/8"}, []string{"10.0.0.0/8"}),
		dev("n2", "b", []string{"10.0.0.0/8"}, nil),
	}
	got := FindConflicts(devices)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(got))
	}
	if n := len(got[0].ActiveHolders()); n != 1 {
		t.Errorf("expected 1 active holder, got %d", n)
	}
}

func TestFindConflictsSameDeviceNoSelfConflict(t *testing.T) {
	devices := []Device{
		dev("n1", "a", []string{"192.168.1.0/24", "192.168.1.0/25"}, []string{"192.168.1.0/24"}),
	}
	if got := FindConflicts(devices); len(got) != 0 {
		t.Errorf("a device must not conflict with itself, got %+v", got)
	}
}

func TestFindConflictsSkipsMalformed(t *testing.T) {
	devices := []Device{
		dev("n1", "a", []string{"not-a-cidr"}, nil),
		dev("n2", "b", []string{"192.168.1.0/24"}, []string{"192.168.1.0/24"}),
	}
	if got := FindConflicts(devices); len(got) != 0 {
		t.Errorf("malformed route must be skipped, got %+v", got)
	}
}

func withEndpoints(d Device, endpoints ...string) Device {
	d.ClientConnectivity = ClientConnectivity{Endpoints: endpoints}
	return d
}

func TestEgressIPsFiltersNonPublic(t *testing.T) {
	d := withEndpoints(Device{},
		"88.186.186.131:41641",    // public, kept
		"192.168.49.26:41641",     // private, skipped
		"100.92.160.72:41641",     // tailnet CGNAT, skipped
		"[2a01:e0a:ef4::1]:41641", // IPv6, skipped
		"88.186.186.131:64640",    // duplicate of the first
	)
	got := d.EgressIPs()
	if len(got) != 1 || got[0] != "88.186.186.131" {
		t.Errorf("EgressIPs() = %v, want [88.186.186.131]", got)
	}
}

func TestEgressIPsEmptyWhenNoEndpoints(t *testing.T) {
	if got := (Device{}).EgressIPs(); len(got) != 0 {
		t.Errorf("expected no egress IPs, got %v", got)
	}
}

func TestConflictHintSameEgress(t *testing.T) {
	// Two routers behind one connection: redundancy, not a mistake.
	devices := []Device{
		withEndpoints(dev("n1", "nas", []string{"192.168.0.0/24"}, []string{"192.168.0.0/24"}),
			"109.190.141.7:41641"),
		withEndpoints(dev("n2", "ha", []string{"192.168.0.0/24"}, []string{"192.168.0.0/24"}),
			"109.190.141.7:12345"),
	}
	got := FindConflicts(devices)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(got))
	}
	if got[0].Hint != HintSameEgress {
		t.Errorf("Hint = %q, want %q", got[0].Hint, HintSameEgress)
	}
}

func TestConflictHintDifferentEgress(t *testing.T) {
	// Two sites reusing the same address plan: a real conflict.
	devices := []Device{
		withEndpoints(dev("n1", "hangar", []string{"192.168.1.0/24"}, []string{"192.168.1.0/24"}),
			"88.186.186.131:41641"),
		withEndpoints(dev("n2", "nas", []string{"192.168.1.0/24"}, []string{"192.168.1.0/24"}),
			"109.190.217.178:41641"),
	}
	got := FindConflicts(devices)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(got))
	}
	if got[0].Hint != HintDifferentEgress {
		t.Errorf("Hint = %q, want %q", got[0].Hint, HintDifferentEgress)
	}
}

func TestConflictHintUnknownWithoutEndpoints(t *testing.T) {
	// An offline device reports no endpoints; guessing would be worse than
	// admitting we cannot tell.
	devices := []Device{
		withEndpoints(dev("n1", "a", []string{"10.0.0.0/8"}, []string{"10.0.0.0/8"}),
			"88.186.186.131:41641"),
		dev("n2", "b", []string{"10.0.0.0/8"}, []string{"10.0.0.0/8"}),
	}
	got := FindConflicts(devices)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(got))
	}
	if got[0].Hint != HintUnknown {
		t.Errorf("Hint = %q, want %q", got[0].Hint, HintUnknown)
	}
}

func TestConflictHintPrivateOnlyEndpointsAreUnknown(t *testing.T) {
	// Endpoints exist but none are public: still inconclusive.
	devices := []Device{
		withEndpoints(dev("n1", "a", []string{"10.0.0.0/8"}, []string{"10.0.0.0/8"}),
			"192.168.1.5:41641"),
		withEndpoints(dev("n2", "b", []string{"10.0.0.0/8"}, []string{"10.0.0.0/8"}),
			"192.168.1.6:41641"),
	}
	got := FindConflicts(devices)
	if len(got) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(got))
	}
	if got[0].Hint != HintUnknown {
		t.Errorf("Hint = %q, want %q", got[0].Hint, HintUnknown)
	}
}

func TestToggleEnable(t *testing.T) {
	got, changed, err := Toggle([]string{"0.0.0.0/0"}, "192.168.1.0/24", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed = true")
	}
	if len(got) != 2 || !contains(got, "192.168.1.0/24") {
		t.Errorf("got %v, want the route added", got)
	}
}

func TestToggleDisable(t *testing.T) {
	got, changed, err := Toggle([]string{"0.0.0.0/0", "192.168.1.0/24", "::/0"}, "192.168.1.0/24", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed = true")
	}
	if contains(got, "192.168.1.0/24") {
		t.Errorf("route still present: %v", got)
	}
	// The exit node must survive an unrelated removal.
	if !contains(got, "0.0.0.0/0") || !contains(got, "::/0") {
		t.Errorf("exit node routes were lost: %v", got)
	}
}

func TestToggleNoop(t *testing.T) {
	if _, changed, _ := Toggle([]string{"192.168.1.0/24"}, "192.168.1.0/24", true); changed {
		t.Error("enabling an already-enabled route must report no change")
	}
	if _, changed, _ := Toggle([]string{"0.0.0.0/0"}, "192.168.1.0/24", false); changed {
		t.Error("disabling an absent route must report no change")
	}
}

func TestToggleNormalizesHostBits(t *testing.T) {
	// 192.168.1.62/24 and 192.168.1.0/24 are the same network.
	got, changed, err := Toggle([]string{"192.168.1.0/24"}, "192.168.1.62/24", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected the equivalent prefix to be matched")
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestToggleInvalidCIDR(t *testing.T) {
	if _, _, err := Toggle(nil, "nope", true); err == nil {
		t.Error("expected an error for an invalid CIDR")
	}
}
