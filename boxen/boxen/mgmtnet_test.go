package boxen_test

import (
	"testing"

	"github.com/carlmontanari/boxen/boxen/boxen"
	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/google/go-cmp/cmp"
)

func TestZipPlatformProfileNats(t *testing.T) {
	tests := []struct {
		desc        string
		tcp         []int
		udp         []int
		wantTCPNats []*config.NatPortPair
		wantUDPNats []*config.NatPortPair
	}{
		{
			desc: "two tcp ports and one udp",
			tcp:  []int{22, 23},
			udp:  []int{161},
			wantTCPNats: []*config.NatPortPair{
				{
					InstanceSide: 22,
					HostSide:     48000,
				},
				{
					InstanceSide: 23,
					HostSide:     48001,
				},
			},
			wantUDPNats: []*config.NatPortPair{
				{
					InstanceSide: 161,
					HostSide:     48002,
				},
			},
		},
		{
			desc: "two tcp ports and no udp",
			tcp:  []int{22, 23},
			wantTCPNats: []*config.NatPortPair{
				{
					InstanceSide: 22,
					HostSide:     48000,
				},
				{
					InstanceSide: 23,
					HostSide:     48001,
				},
			},
			wantUDPNats: []*config.NatPortPair{},
		},
		{
			desc:        "no tcp ports and two udp",
			udp:         []int{161, 200},
			wantTCPNats: []*config.NatPortPair{},
			wantUDPNats: []*config.NatPortPair{
				{
					InstanceSide: 161,
					HostSide:     48000,
				},
				{
					InstanceSide: 200,
					HostSide:     48001,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tcp, udp := boxen.ZipPlatformProfileNats(tt.tcp, tt.udp)

			if !cmp.Equal(tcp, tt.wantTCPNats) {
				t.Fatalf(
					"%s: actual and expected inputs do not match\nactual: %+v\nexpected:%+v",
					tt.desc,
					tcp,
					tt.wantTCPNats,
				)
			}

			if !cmp.Equal(udp, tt.wantUDPNats) {
				t.Fatalf(
					"%s: actual and expected inputs do not match\nactual: %+v\nexpected:%+v",
					tt.desc,
					udp,
					tt.wantUDPNats,
				)
			}
		},
		)
	}
}
