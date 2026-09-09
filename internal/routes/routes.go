// Package routes provides subnet route analysis for Tailscale devices.
//
// Tailscale only ever routes a given subnet through one device. When several
// devices advertise overlapping prefixes, the choice is opaque from the admin
// console, so this package surfaces those overlaps explicitly.
package routes

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// ExitNodeRoutes are the prefixes a device advertises to act as an exit node.
// They overlap every other route by definition, so conflict detection skips them.
var ExitNodeRoutes = []string{"0.0.0.0/0", "::/0"}

// Device is the subset of the Tailscale device API used for route analysis.
type Device struct {
	NodeID           string   `json:"nodeId"`
	Name             string   `json:"name"`
	Hostname         string   `json:"hostname"`
	Tags             []string `json:"tags"`
	AdvertisedRoutes []string `json:"advertisedRoutes"`
	EnabledRoutes    []string `json:"enabledRoutes"`
	ConnectedToProxy bool     `json:"connectedToControl"`
}

// ShortName returns the device name without its tailnet suffix.
func (d Device) ShortName() string {
	if i := strings.IndexByte(d.Name, '.'); i > 0 {
		return d.Name[:i]
	}
	if d.Name != "" {
		return d.Name
	}
	return d.Hostname
}

// IsExitNodeRoute reports whether cidr is one of the exit node prefixes.
func IsExitNodeRoute(cidr string) bool {
	for _, r := range ExitNodeRoutes {
		if cidr == r {
			return true
		}
	}
	return false
}

// Holder is one device advertising or enabling a prefix.
type Holder struct {
	NodeID  string `json:"nodeId"`
	Name    string `json:"name"`
	Route   string `json:"route"`
	Enabled bool   `json:"enabled"`
	Online  bool   `json:"online"`
}

// Conflict is a set of overlapping prefixes held by two or more devices.
//
// Only one holder can actually carry the traffic; Enabled marks which ones are
// currently approved and therefore competing.
type Conflict struct {
	Prefix  string   `json:"prefix"`
	Holders []Holder `json:"holders"`
}

// ActiveHolders returns the holders whose route is approved. More than one means
// the conflict is live rather than latent.
func (c Conflict) ActiveHolders() []Holder {
	var out []Holder
	for _, h := range c.Holders {
		if h.Enabled {
			out = append(out, h)
		}
	}
	return out
}

// subnetRoutes returns the device's non-exit-node prefixes, advertised or enabled.
func subnetRoutes(d Device) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{d.AdvertisedRoutes, d.EnabledRoutes} {
		for _, r := range list {
			if IsExitNodeRoute(r) || seen[r] {
				continue
			}
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// FindConflicts returns every prefix carried by more than one device, whether
// through an exact duplicate or an overlap (a /25 inside a /24, say).
//
// Exit node prefixes are excluded: they overlap everything and are approved per
// device by design. Prefixes that fail to parse are skipped rather than fatal,
// so one malformed entry cannot mask the rest of the report.
func FindConflicts(devices []Device) []Conflict {
	type entry struct {
		dev    Device
		route  string
		prefix netip.Prefix
	}

	var entries []entry
	for _, d := range devices {
		for _, r := range subnetRoutes(d) {
			p, err := netip.ParsePrefix(r)
			if err != nil {
				continue
			}
			entries = append(entries, entry{dev: d, route: r, prefix: p.Masked()})
		}
	}

	// Group by prefix, then fold in any other entry that overlaps it.
	groups := map[string]map[string]Holder{}
	for i, a := range entries {
		for j, b := range entries {
			if i == j || a.dev.NodeID == b.dev.NodeID {
				continue
			}
			if !a.prefix.Overlaps(b.prefix) {
				continue
			}
			key := a.route
			if groups[key] == nil {
				groups[key] = map[string]Holder{}
			}
			for _, e := range []entry{a, b} {
				groups[key][e.dev.NodeID+"|"+e.route] = Holder{
					NodeID:  e.dev.NodeID,
					Name:    e.dev.ShortName(),
					Route:   e.route,
					Enabled: contains(e.dev.EnabledRoutes, e.route),
					Online:  e.dev.ConnectedToProxy,
				}
			}
		}
	}

	out := make([]Conflict, 0, len(groups))
	for prefix, holders := range groups {
		list := make([]Holder, 0, len(holders))
		for _, h := range holders {
			list = append(list, h)
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Name != list[j].Name {
				return list[i].Name < list[j].Name
			}
			return list[i].Route < list[j].Route
		})
		out = append(out, Conflict{Prefix: prefix, Holders: list})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Prefix < out[j].Prefix })
	return out
}

// Toggle returns the enabled route list with cidr added or removed.
//
// The Tailscale API replaces the whole set on write, so callers must send the
// full list back; this computes it from the device's current state. The returned
// bool reports whether the set actually changed.
func Toggle(current []string, cidr string, enable bool) ([]string, bool, error) {
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, false, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	normalized := p.Masked().String()

	out := make([]string, 0, len(current)+1)
	found := false
	for _, r := range current {
		match := r == cidr || r == normalized
		if !match {
			if q, err := netip.ParsePrefix(r); err == nil && q.Masked() == p.Masked() {
				match = true
			}
		}
		if match {
			found = true
			if !enable {
				continue // drop it
			}
		}
		out = append(out, r)
	}

	if enable && !found {
		out = append(out, cidr)
	}

	// Enabling something already present, or removing something absent, is a no-op.
	changed := enable != found

	sort.Strings(out)
	return out, changed, nil
}
