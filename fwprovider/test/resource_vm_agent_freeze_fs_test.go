//go:build acceptance || all

//testacc:tier=medium
//testacc:resource=vm

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package test

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccResourceVMAgentFreezeFS(t *testing.T) {
	t.Parallel()

	te := InitEnvironment(t)
	testVMID := 100000 + rand.Intn(99999)
	te.AddTemplateVars(map[string]any{"TestVMID": testVMID})

	config := func(attrs string) string {
		return te.RenderConfig(`
			resource "proxmox_virtual_environment_vm" "test_agent_freeze_fs" {
				node_name = "{{.NodeName}}"
				vm_id     = {{.TestVMID}}
				name      = "test-agent-freeze-fs"
				started   = false
				agent {
					enabled = true
					` + attrs + `
				}
			}`)
	}

	const res = "proxmox_virtual_environment_vm.test_agent_freeze_fs"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: te.AccProviders,
		Steps: []resource.TestStep{
			{
				Config: config(`freeze_fs = false`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "agent.0.freeze_fs", "false"),
					checkVMAgentFreezeFSOnPVE(te, res, "0"),
				),
			},
			{
				// An unrelated agent change rewrites the agent property; freeze-fs must survive it.
				Config: config(`freeze_fs = false
					trim      = true`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "agent.0.trim", "true"),
					checkVMAgentFreezeFSOnPVE(te, res, "0"),
				),
			},
			{
				Config: config(`trim = true`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "agent.0.freeze_fs", "true"),
					checkVMAgentFreezeFSOnPVE(te, res, ""),
				),
			},
			{
				// Set out of band with the current PVE name: shows as drift instead of being dropped silently.
				PreConfig: func() {
					te.ExecuteNodeCommands([]string{
						fmt.Sprintf("qm set %d --agent enabled=1,fstrim_cloned_disks=1,freeze-fs=0", testVMID),
					})
				},
				Config:             config(`trim = true`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config(`freeze_fs = false
					trim      = true`),
				Check: checkVMAgentFreezeFSOnPVE(te, res, "0"),
			},
			{
				ResourceName:      res,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s/%d", te.NodeName, testVMID),
				ImportStateVerify: true,
			},
		},
	})
}

// checkVMAgentFreezeFSOnPVE asserts the freeze-fs agent setting on PVE: "0", "1", or "" for unset.
func checkVMAgentFreezeFSOnPVE(te *Environment, resourceName string, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceName)
		}

		vmID, err := strconv.Atoi(rs.Primary.Attributes["vm_id"])
		if err != nil {
			return fmt.Errorf("failed to parse vm_id: %w", err)
		}

		vmConfig, err := te.NodeClient().VM(vmID).GetVM(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get VM config: %w", err)
		}

		got := ""
		if vmConfig.Agent != nil && vmConfig.Agent.FreezeFS != nil {
			got = "0"
			if *vmConfig.Agent.FreezeFS {
				got = "1"
			}
		}

		if got != want {
			return fmt.Errorf("unexpected freeze-fs on PVE: got %q, want %q", got, want)
		}

		return nil
	}
}
