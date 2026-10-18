//go:build acceptance || all

//testacc:tier=medium
//testacc:resource=vm

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vm_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/bpg/terraform-provider-proxmox/fwprovider/test"
)

func TestAccResourceVM2PowerStateOnBoot(t *testing.T) {
	t.Parallel()

	te := test.InitEnvironment(t)
	vmID := 100000 + rand.Intn(99999)
	te.AddTemplateVars(map[string]any{"VMID": vmID})

	// stop_on_destroy: the test VM has no OS, so it ignores an ACPI shutdown.
	vm := func(attrs string) string {
		return te.RenderConfig(`
			resource "proxmox_vm" "test_vm" {
				node_name       = "{{.NodeName}}"
				id              = {{.VMID}}
				name            = "test-power-state"
				stop_on_destroy = true
				memory          = { size = 256 }
				timeouts        = { update = "2m" }
				` + attrs + `
			}`)
	}

	const res = "proxmox_vm.test_vm"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: te.AccProviders,
		Steps: []resource.TestStep{
			{
				Config: vm(`power_state = "stopped"`),
				Check: test.ResourceAttributes(res, map[string]string{
					"power_state": "stopped",
					"status":      "stopped",
				}),
			},
			{
				Config: vm(`on_boot = true`),
				Check: test.ResourceAttributes(res, map[string]string{
					"power_state": "running",
					"status":      "running",
					"on_boot":     "true",
				}),
			},
			{
				// Stopped outside Terraform: drift, and apply starts it again.
				PreConfig: func() {
					te.ExecuteNodeCommands([]string{fmt.Sprintf("qm stop %d", vmID)})
				},
				Config:             vm(`on_boot = true`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: vm(`on_boot = true`),
				Check:  test.ResourceAttributes(res, map[string]string{"status": "running"}),
			},
			{
				// Removing on_boot deletes the key; the VM stays running.
				Config: vm(`on_boot = false`),
				Check: test.ResourceAttributes(res, map[string]string{
					"on_boot": "false",
					"status":  "running",
				}),
			},
			{
				Config: vm(""),
				Check: resource.ComposeTestCheckFunc(
					test.NoResourceAttributesSet(res, []string{"on_boot"}),
					test.ResourceAttributes(res, map[string]string{"status": "running"}),
				),
			},
			{
				Config: vm(`power_state = "stopped"`),
				Check:  test.ResourceAttributes(res, map[string]string{"status": "stopped"}),
			},
			{
				ResourceName:            res,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateIdPrefix:     te.NodeName + "/",
				ImportStateVerifyIgnore: []string{"stop_on_destroy"},
			},
		},
	})
}

func TestAccResourceVM2PowerStateTemplate(t *testing.T) {
	t.Parallel()

	te := test.InitEnvironment(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: te.AccProviders,
		Steps: []resource.TestStep{{
			Config: te.RenderConfig(`
				resource "proxmox_vm" "test_template" {
					node_name = "{{.NodeName}}"
					name      = "test-power-state-template"
					template  = true
				}`),
			Check: test.ResourceAttributes("proxmox_vm.test_template", map[string]string{
				"power_state": "stopped",
				"status":      "stopped",
			}),
		}},
	})
}
