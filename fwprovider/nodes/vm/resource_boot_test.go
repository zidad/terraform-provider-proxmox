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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/bpg/terraform-provider-proxmox/fwprovider/test"
)

func TestAccResourceVM2BiosMachineBootOrder(t *testing.T) {
	t.Parallel()

	te := test.InitEnvironment(t)

	tests := []struct {
		name  string
		steps []resource.TestStep
	}{
		{"no bios, machine or boot_order", []resource.TestStep{{
			Config: te.RenderConfig(`
			resource "proxmox_vm" "test_vm" {
				node_name = "{{.NodeName}}"
				name      = "test-boot"
			}`),
			Check: test.NoResourceAttributesSet("proxmox_vm.test_vm", []string{
				"bios",
				"machine",
				"boot_order.#",
			}),
		}}},
		{"set, update, remove, import", []resource.TestStep{
			{
				Config: te.RenderConfig(`
				resource "proxmox_vm" "test_vm" {
					node_name  = "{{.NodeName}}"
					name       = "test-boot"
					cdrom = {
						"ide2" = { file_id = "none" }
						"ide3" = { file_id = "none" }
					}
					bios       = "ovmf"
					machine    = "q35"
					boot_order = ["ide2", "ide3"]
				}`),
				Check: test.ResourceAttributes("proxmox_vm.test_vm", map[string]string{
					"bios":         "ovmf",
					"machine":      "q35",
					"boot_order.#": "2",
					"boot_order.0": "ide2",
					"boot_order.1": "ide3",
				}),
			},
			{
				Config: te.RenderConfig(`
				resource "proxmox_vm" "test_vm" {
					node_name  = "{{.NodeName}}"
					name       = "test-boot"
					cdrom = {
						"ide2" = { file_id = "none" }
						"ide3" = { file_id = "none" }
					}
					bios       = "seabios"
					machine    = "pc"
					boot_order = ["ide3"]
				}`),
				Check: test.ResourceAttributes("proxmox_vm.test_vm", map[string]string{
					"bios":         "seabios",
					"machine":      "pc",
					"boot_order.#": "1",
					"boot_order.0": "ide3",
				}),
			},
			{ // bios and machine are deleted; boot_order is kept (PVE-derived, Optional+Computed)
				Config: te.RenderConfig(`
				resource "proxmox_vm" "test_vm" {
					node_name = "{{.NodeName}}"
					name      = "test-boot"
					cdrom = {
						"ide2" = { file_id = "none" }
						"ide3" = { file_id = "none" }
					}
				}`),
				Check: resource.ComposeTestCheckFunc(
					test.NoResourceAttributesSet("proxmox_vm.test_vm", []string{
						"bios",
						"machine",
					}),
					test.ResourceAttributes("proxmox_vm.test_vm", map[string]string{
						"boot_order.#": "1",
						"boot_order.0": "ide3",
					}),
				),
			},
			{
				ResourceName:        "proxmox_vm.test_vm",
				ImportState:         true,
				ImportStateVerify:   true,
				ImportStateIdPrefix: te.NodeName + "/",
			},
		}},
		{"datasource", []resource.TestStep{{
			Config: te.RenderConfig(`
			resource "proxmox_vm" "test_vm" {
				node_name  = "{{.NodeName}}"
				name       = "test-boot"
				bios       = "ovmf"
				machine    = "q35"
				boot_order = ["ide2"]
				cdrom = {
					"ide2" = { file_id = "none" }
				}
			}
			data "proxmox_vm" "test_vm" {
				node_name = "{{.NodeName}}"
				id        = proxmox_vm.test_vm.id
			}`),
			Check: test.ResourceAttributes("data.proxmox_vm.test_vm", map[string]string{
				"bios":         "ovmf",
				"machine":      "q35",
				"boot_order.#": "1",
				"boot_order.0": "ide2",
			}),
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: te.AccProviders,
				Steps:                    tt.steps,
			})
		})
	}
}
