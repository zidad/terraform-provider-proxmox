//go:build acceptance || all

//testacc:tier=medium
//testacc:resource=vm

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package memory_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/bpg/terraform-provider-proxmox/fwprovider/test"
)

func TestAccResourceVM2Memory(t *testing.T) {
	t.Parallel()

	te := test.InitEnvironment(t)

	vm := func(block string) string {
		return te.RenderConfig(`
			resource "proxmox_vm" "test_vm" {
				node_name   = "{{.NodeName}}"
				name        = "test-memory"
				power_state = "stopped"
				` + block + `
			}`)
	}

	tests := []struct {
		name  string
		steps []resource.TestStep
	}{
		{"no memory block — block stays null", []resource.TestStep{{
			Config: vm(""),
			Check: test.NoResourceAttributesSet("proxmox_vm.test_vm", []string{
				"memory.size", "memory.balloon", "memory.shares", "memory.hugepages", "memory.keep_hugepages", "memory.allow_ksm",
			}),
		}}},
		{"subset, update, zero balloon, removal", []resource.TestStep{
			{
				Config: vm(`memory = { size = 1024 }`),
				Check: resource.ComposeTestCheckFunc(
					test.ResourceAttributes("proxmox_vm.test_vm", map[string]string{"memory.size": "1024"}),
					test.NoResourceAttributesSet("proxmox_vm.test_vm", []string{"memory.balloon", "memory.allow_ksm"}),
				),
			},
			{
				Config: vm(`memory = {
					size      = 2048
					balloon   = 0
					allow_ksm = false
				}`),
				Check: test.ResourceAttributes("proxmox_vm.test_vm", map[string]string{
					"memory.size":      "2048",
					"memory.balloon":   "0",
					"memory.allow_ksm": "false",
				}),
			},
			{
				Config: vm(`memory = {
					size    = 2048
					balloon = 0
				}`),
				Check: resource.ComposeTestCheckFunc(
					test.ResourceAttributes("proxmox_vm.test_vm", map[string]string{"memory.size": "2048", "memory.balloon": "0"}),
					test.NoResourceAttributesSet("proxmox_vm.test_vm", []string{"memory.allow_ksm"}),
				),
			},
			{
				Config: vm(""),
				Check: test.NoResourceAttributesSet("proxmox_vm.test_vm", []string{
					"memory.size", "memory.balloon", "memory.allow_ksm",
				}),
			},
		}},
		{"import", []resource.TestStep{
			{
				Config: vm(`memory = {
					size      = 1024
					allow_ksm = false
				}`),
			},
			{
				ResourceName:        "proxmox_vm.test_vm",
				ImportState:         true,
				ImportStateVerify:   true,
				ImportStateIdPrefix: te.NodeName + "/",
			},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: te.AccProviders,
				Steps:                    tt.steps,
			})
		})
	}
}
