/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vm

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bpg/terraform-provider-proxmox/fwprovider/attribute"
	"github.com/bpg/terraform-provider-proxmox/proxmox/nodes/vms"
)

func bootOrderValue(ctx context.Context, boot *vms.CustomBoot, diags *diag.Diagnostics) types.List {
	if boot == nil || boot.Order == nil {
		return types.ListNull(types.StringType)
	}

	v, d := types.ListValueFrom(ctx, types.StringType, *boot.Order)
	diags.Append(d...)

	return v
}

func bootOrderBody(ctx context.Context, v types.List, diags *diag.Diagnostics) *vms.CustomBoot {
	if !attribute.IsDefined(v) {
		return nil
	}

	var order []string

	diags.Append(v.ElementsAs(ctx, &order, false)...)

	return &vms.CustomBoot{Order: &order}
}

// bootDevicePaths are the device blocks PVE adds to or drops from `boot` on its own.
func bootDevicePaths() []path.Path {
	return []path.Path{path.Root("cdrom")}
}

// bootOrderFollowsDevices keeps the state value of an unset boot_order unless a boot
// device block changes, in which case PVE may rewrite the order and the plan is unknown.
type bootOrderFollowsDevices struct{}

func (bootOrderFollowsDevices) Description(_ context.Context) string {
	return "Uses the prior boot order unless a boot device block changes."
}

func (m bootOrderFollowsDevices) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (bootOrderFollowsDevices) PlanModifyList(
	ctx context.Context,
	req planmodifier.ListRequest,
	resp *planmodifier.ListResponse,
) {
	if req.State.Raw.IsNull() || !req.ConfigValue.IsNull() {
		return
	}

	for _, p := range bootDevicePaths() {
		var planDev, stateDev attr.Value

		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, p, &planDev)...)
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, p, &stateDev)...)

		if resp.Diagnostics.HasError() {
			return
		}

		if !planDev.Equal(stateDev) {
			resp.PlanValue = types.ListUnknown(types.StringType)
			return
		}
	}

	resp.PlanValue = req.StateValue
}
