package provider

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// hashForState computes the hexadecimal representation of the SHA1 checksum of a string.
// This is used by most resources/data-sources here to compute their Unique Identifier (ID).
func hashForState(value string) string {
	if value == "" {
		return ""
	}
	hash := sha1.Sum([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(hash[:])
}

// overridableTimeFunc normally returns time.Now(),
// but it is overridden during testing to simulate an arbitrary value of "now".
var overridableTimeFunc = func() time.Time {
	return time.Now()
}

// updatedUsingPlan is to be used as part of resource.Resource `Update`.
// It takes the resource.UpdateRequest `Plan` and sets it on resource.UpdateResponse State.
//
// Use this if the planned values should just be copied over into the new state.
func updatedUsingPlan(ctx context.Context, req *resource.UpdateRequest, res *resource.UpdateResponse, model interface{}) {
	// Read the plan
	res.Diagnostics.Append(req.Plan.Get(ctx, model)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Set it as the new state
	res.Diagnostics.Append(res.State.Set(ctx, model)...)
}

// requireReplaceIfStateContainsPEMString returns a tfsdk.AttributePlanModifier that triggers a
// replacement of the resource if (and only if) all the conditions of a resource.RequiresReplace are met,
// and the attribute value is a PEM string.
func requireReplaceIfStateContainsPEMString() tfsdk.AttributePlanModifier {
	description := "Attribute requires replacement if it contains a PEM string"

	return resource.RequiresReplaceIf(func(ctx context.Context, state, _ attr.Value, path path.Path) (bool, diag.Diagnostics) {
		// NOTE: If we reach this point, we know a change has been detected and that is known AND not-null

		// First, we verify the type is a String, as expected
		stateType := state.Type(ctx)
		if stateType != types.StringType {
			return false, diag.Diagnostics{
				diag.NewAttributeErrorDiagnostic(
					path,
					fmt.Sprintf("Failed to determine if resource requires replacement: expected %q, got %q", types.StringType, stateType),
					"This is a bug with the provider, and should be reported to their issue tracker.",
				),
			}
		}

		stateValue := state.(types.String).ValueString()

		// If the value is indeed a PEM, and
		if regexp.MustCompile(`^-----BEGIN [[:alpha:] ]+-----\n(.|\s)+\n-----END [[:alpha:] ]+-----\n?$`).MatchString(stateValue) {
			return true, nil
		}

		return false, nil
	}, description, description)
}
