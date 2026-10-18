/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vm

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bpg/terraform-provider-proxmox/fwprovider/migration"
	"github.com/bpg/terraform-provider-proxmox/fwprovider/nodes/vm/cdrom"
	"github.com/bpg/terraform-provider-proxmox/fwprovider/nodes/vm/cpu"
	"github.com/bpg/terraform-provider-proxmox/fwprovider/nodes/vm/rng"
	"github.com/bpg/terraform-provider-proxmox/fwprovider/nodes/vm/vga"
	"github.com/bpg/terraform-provider-proxmox/fwprovider/types/stringset"
)

// Schema defines the schema for the resource.
func (r *Resource) Schema(
	ctx context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		DeprecationMessage: migration.DeprecationMessage("proxmox_vm"),
		Description:        "This is an experimental implementation of a Proxmox VM resource using Plugin Framework.",
		MarkdownDescription: "This is an experimental implementation of a Proxmox VM resource using Plugin Framework." +
			"<br><br>It is a Proof of Concept, highly experimental and **will** change in future. " +
			"It does not support all features of the Proxmox API for VMs and **MUST NOT** be used in production.",
		Attributes: map[string]schema.Attribute{
			"bios": schema.StringAttribute{
				Description: "The BIOS implementation, e.g. `seabios` or `ovmf`. PVE uses `seabios` when unset.",
				MarkdownDescription: "The BIOS implementation, e.g. `seabios` or `ovmf`. PVE uses `seabios` when unset. " +
					"A change applies at the next VM start.",
				Optional: true,
			},
			"boot_order": schema.ListAttribute{
				Description: "The boot order as a list of device slots, e.g. `[\"scsi0\", \"ide2\", \"net0\"]`.",
				MarkdownDescription: "The boot order as a list of device slots, e.g. `[\"scsi0\", \"ide2\", \"net0\"]`. " +
					"When unset, PVE derives it and rewrites it as devices are added or removed; removing the attribute keeps the current order.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.List{
					bootOrderFollowsDevices{},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
			},
			"cdrom": cdrom.ResourceSchema(),
			"cpu":   cpu.ResourceSchema(),
			"description": schema.StringAttribute{
				Description: "The description of the VM.",
				Optional:    true,
			},
			"id": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
				Description: "The unique identifier of the VM in the Proxmox cluster.",
			},
			"machine": schema.StringAttribute{
				Description: "The machine type, e.g. `q35` or `pc-i440fx-10.0`. PVE uses `i440fx` when unset.",
				MarkdownDescription: "The machine type, e.g. `q35` or `pc-i440fx-10.0`. PVE uses `i440fx` when unset. " +
					"A change applies at the next VM start.",
				Optional: true,
			},
			"name": schema.StringAttribute{
				Description:         "The name of the VM.",
				MarkdownDescription: "The name of the VM. Doesn't have to be unique.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`),
						"must be a valid DNS name",
					),
				},
			},
			"node_name": schema.StringAttribute{
				Description: "The name of the node where the VM is provisioned.",
				Required:    true,
			},
			"rng": rng.ResourceSchema(),
			"stop_on_destroy": schema.BoolAttribute{
				Description:         "Set to true to stop (rather than shutdown) the VM on destroy.",
				MarkdownDescription: "Set to true to stop (rather than shutdown) the VM on destroy (defaults to `false`).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"purge_on_destroy": schema.BoolAttribute{
				Description:         "Set to true to purge the VM from backup configurations on destroy.",
				MarkdownDescription: "Set to true to purge the VM from backup configurations on destroy (defaults to `true`).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"delete_unreferenced_disks_on_destroy": schema.BoolAttribute{
				Description:         "Set to true to delete unreferenced disks on destroy.",
				MarkdownDescription: "Set to true to delete unreferenced disks on destroy (defaults to `true`).",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"tags": stringset.ResourceAttribute("The tags assigned to the VM.", ""),
			"template": schema.BoolAttribute{
				Description: "Set to true to create a VM template.",
				Optional:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			}),
			"vga": vga.ResourceSchema(),
		},
	}
}
