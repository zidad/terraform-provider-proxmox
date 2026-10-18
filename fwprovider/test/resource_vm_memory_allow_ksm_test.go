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

func TestAccResourceVMMemoryAllowKSM(t *testing.T) {
	t.Parallel()

	te := InitEnvironment(t)
	testVMID := 100000 + rand.Intn(99999)
	te.AddTemplateVars(map[string]any{"TestVMID": testVMID})

	config := func(attrs string) string {
		return te.RenderConfig(`
			resource "proxmox_virtual_environment_vm" "test_memory_allow_ksm" {
				node_name = "{{.NodeName}}"
				vm_id     = {{.TestVMID}}
				name      = "test-memory-allow-ksm"
				started   = false
				memory {
					dedicated = 512
					` + attrs + `
				}
			}`)
	}

	const res = "proxmox_virtual_environment_vm.test_memory_allow_ksm"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: te.AccProviders,
		Steps: []resource.TestStep{
			{
				Config: config(`allow_ksm = false`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "memory.0.allow_ksm", "false"),
					checkVMAllowKSMOnPVE(te, res, "0"),
				),
			},
			{
				// An unrelated memory change keeps the setting.
				Config: config(`allow_ksm = false
					floating  = 512`),
				Check: checkVMAllowKSMOnPVE(te, res, "0"),
			},
			{
				// Back to the default deletes the key rather than writing allow-ksm=1.
				Config: config(`floating = 512`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(res, "memory.0.allow_ksm", "true"),
					checkVMAllowKSMOnPVE(te, res, ""),
				),
			},
			{
				PreConfig: func() {
					te.ExecuteNodeCommands([]string{fmt.Sprintf("qm set %d --allow-ksm 0", testVMID)})
				},
				Config:             config(`floating = 512`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config(`allow_ksm = false
					floating  = 512`),
				Check: checkVMAllowKSMOnPVE(te, res, "0"),
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

// checkVMAllowKSMOnPVE asserts allow-ksm on PVE: "0", "1", or "" for unset.
func checkVMAllowKSMOnPVE(te *Environment, resourceName string, want string) resource.TestCheckFunc {
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
		if vmConfig.AllowKSM != nil {
			got = "0"
			if *vmConfig.AllowKSM {
				got = "1"
			}
		}

		if got != want {
			return fmt.Errorf("unexpected allow-ksm on PVE: got %q, want %q", got, want)
		}

		return nil
	}
}
