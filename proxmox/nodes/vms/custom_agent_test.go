/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vms

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bpg/terraform-provider-proxmox/proxmox/types"
)

func TestCustomAgentUnmarshalFreezeFS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want *types.CustomBool
	}{
		{"unset", `"enabled=1"`, nil},
		{"freeze-fs off", `"enabled=1,freeze-fs=0"`, types.CustomBool(false).Pointer()},
		{"freeze-fs on", `"enabled=1,freeze-fs=1"`, types.CustomBool(true).Pointer()},
		{"deprecated freeze-fs-on-backup", `"enabled=1,freeze-fs-on-backup=0"`, types.CustomBool(false).Pointer()},
		{"deprecated guest-fsfreeze", `"1,guest-fsfreeze=0"`, types.CustomBool(false).Pointer()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var a CustomAgent
			require.NoError(t, a.UnmarshalJSON([]byte(tt.line)))
			assert.Equal(t, tt.want, a.FreezeFS)
			require.NotNil(t, a.Enabled)
			assert.True(t, bool(*a.Enabled))
		})
	}
}

func TestCustomAgentEncodeFreezeFS(t *testing.T) {
	t.Parallel()

	enabled := types.CustomBool(true)
	agentType := "virtio"

	tests := []struct {
		name     string
		freezeFS *types.CustomBool
		want     string
	}{
		{"unset keeps the agent string unchanged", nil, "enabled=1,type=virtio"},
		{"off", types.CustomBool(false).Pointer(), "enabled=1,freeze-fs-on-backup=0,type=virtio"},
		{"on", types.CustomBool(true).Pointer(), "enabled=1,freeze-fs-on-backup=1,type=virtio"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := CustomAgent{Enabled: &enabled, FreezeFS: tt.freezeFS, Type: &agentType}
			v := url.Values{}
			require.NoError(t, a.EncodeValues("agent", &v))
			assert.Equal(t, tt.want, v.Get("agent"))
		})
	}
}
