/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package vm

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bpg/terraform-provider-proxmox/proxmox/nodes/vms"
)

const (
	powerStateRunning = "running"
	powerStateStopped = "stopped"

	shutdownDeadlineMargin = 30 * time.Second
)

// powerStateDefault plans `running` when power_state is unset, or `stopped` for templates, which PVE never runs.
type powerStateDefault struct{}

func (powerStateDefault) Description(context.Context) string {
	return "Defaults to `running`, or `stopped` when `template` is true."
}

func (m powerStateDefault) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (powerStateDefault) PlanModifyString(
	ctx context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	if !req.ConfigValue.IsNull() {
		return
	}

	resp.PlanValue = desiredPowerState(ctx, req.Config, &resp.Diagnostics)
}

// desiredPowerState is power_state from config, or its default: `stopped` for templates, else `running`.
// Plan modifiers see the proposed plan, not other attributes' modified values, so both modifiers derive it here.
func desiredPowerState(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) types.String {
	var powerState types.String

	diags.Append(config.GetAttribute(ctx, path.Root("power_state"), &powerState)...)

	if !powerState.IsNull() {
		return powerState
	}

	var template types.Bool

	diags.Append(config.GetAttribute(ctx, path.Root("template"), &template)...)

	if template.ValueBool() {
		return types.StringValue(powerStateStopped)
	}

	return types.StringValue(powerStateRunning)
}

// applyPowerState moves the VM to the planned power state. Pending configuration changes are
// not applied by rebooting a running VM: they take effect on its next start.
func applyPowerState(ctx context.Context, vmAPI *vms.Client, plan Model, diags *diag.Diagnostics) {
	if plan.Template.ValueBool() || plan.PowerState.IsNull() || plan.PowerState.IsUnknown() {
		return
	}

	status, err := vmAPI.GetVMStatus(ctx)
	if err != nil {
		diags.AddError(fmt.Sprintf("Unable to Read VM %d Status", plan.ID.ValueInt64()), err.Error())
		return
	}

	want := plan.PowerState.ValueString()
	if powerStateFromStatus(status.Status) == want {
		return
	}

	switch want {
	case powerStateRunning:
		tflog.Info(ctx, fmt.Sprintf("Starting VM %d", plan.ID.ValueInt64()))

		timeoutSec := int(defaultStartTimeout.Seconds())
		if dl, ok := ctx.Deadline(); ok {
			timeoutSec = int(time.Until(dl).Seconds())
		}

		if vmAPI.StartVM(ctx, timeoutSec).AddDiags(diags, fmt.Sprintf("Unable to Start VM %d", plan.ID.ValueInt64())) {
			return
		}

		if err := vmAPI.WaitForVMStatus(ctx, powerStateRunning); err != nil {
			diags.AddError(fmt.Sprintf("Unable to Start VM %d", plan.ID.ValueInt64()), err.Error())
		}
	case powerStateStopped:
		tflog.Info(ctx, fmt.Sprintf("Stopping VM %d", plan.ID.ValueInt64()))

		// Shutdown force-stops after its timeout, so a guest that ignores ACPI still ends up stopped. Bounded
		// to defaultShutdownTimeout and kept inside the update deadline, leaving time for the forced stop.
		limit := defaultShutdownTimeout
		if dl, ok := ctx.Deadline(); ok && time.Until(dl)-shutdownDeadlineMargin < limit {
			limit = time.Until(dl) - shutdownDeadlineMargin
		}

		shutdownCtx, cancel := context.WithTimeout(ctx, limit)
		defer cancel()

		result := vmShutdown(shutdownCtx, vmAPI)
		if result.Err() != nil {
			if err := vmAPI.WaitForVMStatus(ctx, powerStateStopped); err != nil {
				result.AddDiags(diags, fmt.Sprintf("Unable to Shutdown VM %d", plan.ID.ValueInt64()))
			}
		}
	}
}

// statusFollowsPowerState plans status as unknown when power_state is about to change it, and keeps the prior
// value otherwise. Without it a power-only change plans the old status and apply returns an inconsistent result.
type statusFollowsPowerState struct{}

func (statusFollowsPowerState) Description(context.Context) string {
	return "Unknown when power_state changes, otherwise the prior status."
}

func (m statusFollowsPowerState) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (statusFollowsPowerState) PlanModifyString(
	ctx context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	if req.StateValue.IsNull() {
		return
	}

	var statePower types.String

	planPower := desiredPowerState(ctx, req.Config, &resp.Diagnostics)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("power_state"), &statePower)...)

	if planPower.IsUnknown() || !planPower.Equal(statePower) {
		resp.PlanValue = types.StringUnknown()
		return
	}

	resp.PlanValue = req.StateValue
}
