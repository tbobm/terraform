// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stackruntime

import (
	"context"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/internal/addrs"
	"github.com/hashicorp/terraform/internal/collections"
	"github.com/hashicorp/terraform/internal/depsfile"
	"github.com/hashicorp/terraform/internal/getproviders/providerreqs"
	"github.com/hashicorp/terraform/internal/plans"
	"github.com/hashicorp/terraform/internal/providers"
	"github.com/hashicorp/terraform/internal/stacks/stackaddrs"
	"github.com/hashicorp/terraform/internal/stacks/stackplan"
	stacks_testing_provider "github.com/hashicorp/terraform/internal/stacks/stackruntime/testing"
	"github.com/hashicorp/terraform/internal/stacks/stackstate"
	"github.com/hashicorp/terraform/internal/states"
	"github.com/hashicorp/terraform/internal/tfdiags"
	"github.com/hashicorp/terraform/version"
)

func TestApplyDestroy(t *testing.T) {

	fakePlanTimestamp, err := time.Parse(time.RFC3339, "2021-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	tcs := map[string]struct {
		path        string
		description string
		state       *stackstate.State
		store       *stacks_testing_provider.ResourceStore
		cycles      []TestCycle
	}{
		"inputs-and-outputs": {
			path: "component-input-output",
			state: stackstate.NewStateBuilder().
				AddInput("value", cty.StringVal("foo")).
				AddOutput("value", cty.StringVal("foo")).
				Build(),
			cycles: []TestCycle{
				{
					planMode: plans.DestroyMode,
					planInputs: map[string]cty.Value{
						"value": cty.StringVal("foo"),
					},
					wantPlannedChanges: []stackplan.PlannedChange{
						&stackplan.PlannedChangeApplyable{
							Applyable: true,
						},
						&stackplan.PlannedChangeHeader{
							TerraformVersion: version.SemVer,
						},
						&stackplan.PlannedChangeOutputValue{
							Addr:   mustStackOutputValue("value"),
							Action: plans.Delete,
							Before: cty.StringVal("foo"),
							After:  cty.NullVal(cty.String),
						},
						&stackplan.PlannedChangePlannedTimestamp{
							PlannedTimestamp: fakePlanTimestamp,
						},
						&stackplan.PlannedChangeRootInputValue{
							Addr:          mustStackInputVariable("value"),
							Action:        plans.NoOp,
							Before:        cty.StringVal("foo"),
							After:         cty.StringVal("foo"),
							DeleteOnApply: true,
						},
					},
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeOutputValue{
							Addr:  mustStackOutputValue("value"),
							Value: cty.NilVal, // destroyed
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("value"),
							Removed: true, // destroyed
						},
					},
				},
			},
		},
		"missing-resource": {
			path:        path.Join("with-single-input", "valid"),
			description: "tests what happens when a resource is in state but not in the provider",
			state: stackstate.NewStateBuilder().
				AddComponentInstance(stackstate.NewComponentInstanceBuilder(mustAbsComponentInstance("component.self")).
					AddInputVariable("id", cty.StringVal("e84b59f2")).
					AddInputVariable("value", cty.StringVal("hello"))).
				AddResourceInstance(stackstate.NewResourceInstanceBuilder().
					SetAddr(mustAbsResourceInstanceObject("component.self.testing_resource.data")).
					SetProviderAddr(mustDefaultRootProvider("testing")).
					SetResourceInstanceObjectSrc(states.ResourceInstanceObjectSrc{
						SchemaVersion: 0,
						AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
							"id":    "e84b59f2",
							"value": "hello",
						}),
						Status: states.ObjectReady,
					})).
				Build(),
			cycles: []TestCycle{
				{
					planMode: plans.DestroyMode,
					planInputs: map[string]cty.Value{
						"input": cty.StringVal("hello"),
					},
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.NullVal(cty.String),
								mustInputVariable("input"): cty.StringVal("hello"),
							},
						},
						// The resource that was in state but not in the data store should still
						// be included to be destroyed.
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							NewStateSrc:                nil, // We should be removing this from the state file.
							Schema:                     nil,
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("id"),
							Removed: true,
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("input"),
							Removed: true,
						},
					},
				},
			},
		},
		"datasource-in-state": {
			path:        "with-data-source",
			description: "tests that we emit removal notices for data sources",
			store: stacks_testing_provider.NewResourceStoreBuilder().
				AddResource("foo", cty.ObjectVal(map[string]cty.Value{
					"id":    cty.StringVal("foo"),
					"value": cty.StringVal("hello"),
				})).Build(),
			state: stackstate.NewStateBuilder().
				AddResourceInstance(stackstate.NewResourceInstanceBuilder().
					SetAddr(mustAbsResourceInstanceObject("component.self.data.testing_data_source.missing")).
					SetProviderAddr(mustDefaultRootProvider("testing")).
					SetResourceInstanceObjectSrc(states.ResourceInstanceObjectSrc{
						SchemaVersion: 0,
						AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
							"id":    "e84b59f2",
							"value": "hello",
						}),
						Status: states.ObjectReady,
					})).
				Build(),
			cycles: []TestCycle{
				{
					planMode: plans.DestroyMode,
					planInputs: map[string]cty.Value{
						"id":       cty.StringVal("foo"),
						"resource": cty.StringVal("bar"),
					},
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):       cty.StringVal("foo"),
								mustInputVariable("resource"): cty.StringVal("bar"),
							},
						},
						// This is a bit of a quirk of the system, this wasn't in the state
						// file before so we don't need to emit this. But since Terraform
						// pushes data sources into the refresh state, it's very difficult to
						// tell the difference between this kind of change that doesn't need to
						// be emitted, and the next change that does need to be emitted. It's
						// better to emit both than to miss one, and emitting this doesn't
						// actually harm anything.
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.data.testing_data_source.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							Schema:                     nil,
							NewStateSrc:                nil, // deleted
						},

						// This was in the state file, so we're emitting the destroy notice.
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.data.testing_data_source.missing"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							Schema:                     nil,
							NewStateSrc:                nil,
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("id"),
							Removed: true,
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("resource"),
							Removed: true,
						},
					},
				},
			},
		},
		"orphaned-data-sources-removed": {
			path:        "with-data-source",
			description: "tests that we emit removal notices for data sources that are no longer in the configuration",
			store: stacks_testing_provider.NewResourceStoreBuilder().
				AddResource("foo", cty.ObjectVal(map[string]cty.Value{
					"id":    cty.StringVal("foo"),
					"value": cty.StringVal("hello"),
				})).Build(),
			state: stackstate.NewStateBuilder().
				AddResourceInstance(stackstate.NewResourceInstanceBuilder().
					SetAddr(mustAbsResourceInstanceObject("component.self.data.testing_data_source.missing")).
					SetProviderAddr(mustDefaultRootProvider("testing")).
					SetResourceInstanceObjectSrc(states.ResourceInstanceObjectSrc{
						SchemaVersion: 0,
						AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
							"id":    "e84b59f2",
							"value": "hello",
						}),
						Status: states.ObjectReady,
					})).
				Build(),
			cycles: []TestCycle{
				{
					planMode: plans.NormalMode,
					planInputs: map[string]cty.Value{
						"id":       cty.StringVal("foo"),
						"resource": cty.StringVal("bar"),
					},
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):       cty.StringVal("foo"),
								mustInputVariable("resource"): cty.StringVal("bar"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.data.testing_data_source.data"),
							NewStateSrc: &states.ResourceInstanceObjectSrc{
								AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
									"id":    "foo",
									"value": "hello",
								}),
								AttrSensitivePaths: make([]cty.Path, 0),
								Status:             states.ObjectReady,
							},
							ProviderConfigAddr: mustDefaultRootProvider("testing"),
							Schema:             stacks_testing_provider.TestingDataSourceSchema,
						},
						// This data source should be removed from the state file as it is no
						// longer in the configuration.
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.data.testing_data_source.missing"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							Schema:                     nil,
							NewStateSrc:                nil, // deleted
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.testing_resource.data"),
							NewStateSrc: &states.ResourceInstanceObjectSrc{
								AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
									"id":    "bar",
									"value": "hello",
								}),
								Status: states.ObjectReady,
								Dependencies: []addrs.ConfigResource{
									mustAbsResourceInstance("data.testing_data_source.data").ConfigResource(),
								},
							},
							ProviderConfigAddr: mustDefaultRootProvider("testing"),
							Schema:             stacks_testing_provider.TestingResourceSchema,
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:  mustStackInputVariable("id"),
							Value: cty.StringVal("foo"),
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:  mustStackInputVariable("resource"),
							Value: cty.StringVal("bar"),
						},
					},
				},
				{
					planMode: plans.DestroyMode,
					planInputs: map[string]cty.Value{
						"id":       cty.StringVal("foo"),
						"resource": cty.StringVal("bar"),
					},
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):       cty.StringVal("foo"),
								mustInputVariable("resource"): cty.StringVal("bar"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.data.testing_data_source.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							Schema:                     nil,
							NewStateSrc:                nil, // deleted
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							Schema:                     nil,
							NewStateSrc:                nil, // deleted
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("id"),
							Removed: true,
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("resource"),
							Removed: true,
						},
					},
				},
			},
		},
		"dependent-resources": {
			path:        "dependent-component",
			description: "test the order of operations during create and destroy",
			cycles: []TestCycle{
				{
					planMode: plans.NormalMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.valid")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"): cty.StringVal("dependent"),
								mustInputVariable("requirements"): cty.SetVal([]cty.Value{
									cty.StringVal("valid"),
								}),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.testing_blocked_resource.resource"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							NewStateSrc: &states.ResourceInstanceObjectSrc{
								AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
									"id":                 "dependent",
									"value":              nil,
									"required_resources": []interface{}{"valid"},
								}),
								Status:       states.ObjectReady,
								Dependencies: make([]addrs.ConfigResource, 0),
							},
							Schema: stacks_testing_provider.BlockedResourceSchema,
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.valid"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.valid"),
							Dependents:            collections.NewSet(mustAbsComponent("component.self")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("valid"),
								mustInputVariable("input"): cty.StringVal("resource"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.valid.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							NewStateSrc: &states.ResourceInstanceObjectSrc{
								AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
									"id":    "valid",
									"value": "resource",
								}),
								Status:       states.ObjectReady,
								Dependencies: make([]addrs.ConfigResource, 0),
							},
							Schema: stacks_testing_provider.TestingResourceSchema,
						},
					},
				},
				{
					planMode: plans.DestroyMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.valid")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"): cty.StringVal("dependent"),
								mustInputVariable("requirements"): cty.SetVal([]cty.Value{
									cty.StringVal("valid"),
								}),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.testing_blocked_resource.resource"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.valid"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.valid"),
							Dependents:            collections.NewSet(mustAbsComponent("component.self")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("valid"),
								mustInputVariable("input"): cty.StringVal("resource"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.valid.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
					},
				},
			},
		},
		"failed-destroy": {
			path:        "failed-component",
			description: "tests what happens if a component fails to destroy",
			state: stackstate.NewStateBuilder().
				AddComponentInstance(stackstate.NewComponentInstanceBuilder(mustAbsComponentInstance("component.self"))).
				AddResourceInstance(stackstate.NewResourceInstanceBuilder().
					SetAddr(mustAbsResourceInstanceObject("component.self.testing_failed_resource.data")).
					SetResourceInstanceObjectSrc(states.ResourceInstanceObjectSrc{
						AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
							"id":         "failed",
							"value":      "resource",
							"fail_plan":  false,
							"fail_apply": true,
						}),
						Status: states.ObjectReady,
					}).
					SetProviderAddr(mustDefaultRootProvider("testing"))).
				Build(),
			store: stacks_testing_provider.NewResourceStoreBuilder().
				AddResource("failed", cty.ObjectVal(map[string]cty.Value{
					"id":         cty.StringVal("failed"),
					"value":      cty.StringVal("resource"),
					"fail_plan":  cty.False,
					"fail_apply": cty.True,
				})).
				Build(),
			cycles: []TestCycle{
				{
					planMode: plans.DestroyMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):         cty.StringVal("failed"),
								mustInputVariable("input"):      cty.StringVal("resource"),
								mustInputVariable("fail_plan"):  cty.False,
								mustInputVariable("fail_apply"): cty.False,
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.testing_failed_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							NewStateSrc: &states.ResourceInstanceObjectSrc{
								AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
									"id":         "failed",
									"value":      "resource",
									"fail_plan":  false,
									"fail_apply": true,
								}),
								Status:       states.ObjectReady,
								Dependencies: make([]addrs.ConfigResource, 0),
							},
							Schema: stacks_testing_provider.FailedResourceSchema,
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("fail_apply"),
							Removed: true,
						},
						&stackstate.AppliedChangeInputVariable{
							Addr:    mustStackInputVariable("fail_plan"),
							Removed: true,
						},
					},
					wantAppliedDiags: initDiags(func(diags tfdiags.Diagnostics) tfdiags.Diagnostics {
						return diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "failedResource error",
							Detail:   "failed during apply",
						})
					}),
				},
			},
		},
		"destroy-after-failed-apply": {
			path:        path.Join("with-single-input", "failed-child"),
			description: "tests destroying when state is only partially applied",
			cycles: []TestCycle{
				{
					planMode: plans.NormalMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.child"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.child"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.self")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):         cty.NullVal(cty.String),
								mustInputVariable("input"):      cty.StringVal("child"),
								mustInputVariable("fail_plan"):  cty.NullVal(cty.Bool),
								mustInputVariable("fail_apply"): cty.True,
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.child.testing_failed_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							Dependents:            collections.NewSet(mustAbsComponent("component.child")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("self"),
								mustInputVariable("input"): cty.StringVal("value"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							NewStateSrc: &states.ResourceInstanceObjectSrc{
								AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
									"id":    "self",
									"value": "value",
								}),
								Status:       states.ObjectReady,
								Dependencies: make([]addrs.ConfigResource, 0),
							},
							Schema: stacks_testing_provider.TestingResourceSchema,
						},
					},
					wantAppliedDiags: initDiags(func(diags tfdiags.Diagnostics) tfdiags.Diagnostics {
						return diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "failedResource error",
							Detail:   "failed during apply",
						})
					}),
				},
				{
					planMode: plans.DestroyMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.child"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.child"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.self")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):         cty.NullVal(cty.String),
								mustInputVariable("input"):      cty.StringVal("child"),
								mustInputVariable("fail_plan"):  cty.NullVal(cty.Bool),
								mustInputVariable("fail_apply"): cty.True,
							},
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.self"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.self"),
							Dependents:            collections.NewSet(mustAbsComponent("component.child")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("self"),
								mustInputVariable("input"): cty.StringVal("value"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.self.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
					},
				},
			},
		},
		"destroy-after-deferred-apply": {
			path:        "deferred-dependent",
			description: "tests what happens when a destroy plan is applied after components have been deferred",
			cycles: []TestCycle{
				{
					planMode: plans.NormalMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.deferred"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.deferred"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.valid")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("deferred"),
								mustInputVariable("defer"): cty.True,
							},
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.valid"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.valid"),
							Dependents:            collections.NewSet(mustAbsComponent("component.deferred")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("valid"),
								mustInputVariable("input"): cty.StringVal("valid"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.valid.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
							NewStateSrc: &states.ResourceInstanceObjectSrc{
								AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
									"id":    "valid",
									"value": "valid",
								}),
								Status:       states.ObjectReady,
								Dependencies: make([]addrs.ConfigResource, 0),
							},
							Schema: stacks_testing_provider.TestingResourceSchema,
						},
					},
				},
				{
					planMode: plans.DestroyMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.deferred"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.deferred"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.valid")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("deferred"),
								mustInputVariable("defer"): cty.True,
							},
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.valid"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.valid"),
							Dependents:            collections.NewSet(mustAbsComponent("component.deferred")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("valid"),
								mustInputVariable("input"): cty.StringVal("valid"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.valid.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
					},
				},
			},
		},
		"deferred-destroy": {
			path:        "deferred-dependent",
			description: "tests what happens when a destroy operation is deferred",
			state: stackstate.NewStateBuilder().
				AddComponentInstance(stackstate.NewComponentInstanceBuilder(mustAbsComponentInstance("component.valid")).
					AddDependent(mustAbsComponent("component.deferred"))).
				AddResourceInstance(stackstate.NewResourceInstanceBuilder().
					SetAddr(mustAbsResourceInstanceObject("component.valid.testing_resource.data")).
					SetProviderAddr(mustDefaultRootProvider("testing")).
					SetResourceInstanceObjectSrc(states.ResourceInstanceObjectSrc{
						AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
							"id":    "valid",
							"value": "valid",
						}),
						Status: states.ObjectReady,
					})).
				AddComponentInstance(stackstate.NewComponentInstanceBuilder(mustAbsComponentInstance("component.deferred")).
					AddDependency(mustAbsComponent("component.valid"))).
				AddResourceInstance(stackstate.NewResourceInstanceBuilder().
					SetAddr(mustAbsResourceInstanceObject("component.deferred.testing_deferred_resource.data")).
					SetProviderAddr(mustDefaultRootProvider("testing")).
					SetResourceInstanceObjectSrc(states.ResourceInstanceObjectSrc{
						AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
							"id":       "deferred",
							"value":    nil,
							"deferred": true,
						}),
						Status: states.ObjectReady,
					})).
				Build(),
			store: stacks_testing_provider.NewResourceStoreBuilder().
				AddResource("valid", cty.ObjectVal(map[string]cty.Value{
					"id":    cty.StringVal("valid"),
					"value": cty.StringVal("valid"),
				})).
				AddResource("deferred", cty.ObjectVal(map[string]cty.Value{
					"id":       cty.StringVal("deferred"),
					"value":    cty.NullVal(cty.String),
					"deferred": cty.True,
				})).
				Build(),
			cycles: []TestCycle{
				{
					planMode: plans.DestroyMode,
					wantPlannedChanges: []stackplan.PlannedChange{
						&stackplan.PlannedChangeApplyable{
							Applyable: true,
						},
						&stackplan.PlannedChangeComponentInstance{
							Addr:               mustAbsComponentInstance("component.deferred"),
							Action:             plans.Delete,
							Mode:               plans.DestroyMode,
							RequiredComponents: collections.NewSet[stackaddrs.AbsComponent](mustAbsComponent("component.valid")),
							PlannedInputValues: map[string]plans.DynamicValue{
								"id":    mustPlanDynamicValueDynamicType(cty.StringVal("deferred")),
								"defer": mustPlanDynamicValueDynamicType(cty.True),
							},
							PlannedInputValueMarks: map[string][]cty.PathValueMarks{
								"id":    nil,
								"defer": nil,
							},
							PlannedOutputValues: make(map[string]cty.Value),
							PlannedCheckResults: &states.CheckResults{},
							PlanTimestamp:       fakePlanTimestamp,
						},
						&stackplan.PlannedChangeDeferredResourceInstancePlanned{
							ResourceInstancePlanned: stackplan.PlannedChangeResourceInstancePlanned{
								ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.deferred.testing_deferred_resource.data"),
								ChangeSrc: &plans.ResourceInstanceChangeSrc{
									Addr:         mustAbsResourceInstance("testing_deferred_resource.data"),
									PrevRunAddr:  mustAbsResourceInstance("testing_deferred_resource.data"),
									ProviderAddr: mustDefaultRootProvider("testing"),
									ChangeSrc: plans.ChangeSrc{
										Action: plans.Delete,
										Before: mustPlanDynamicValue(cty.ObjectVal(map[string]cty.Value{
											"id":       cty.StringVal("deferred"),
											"value":    cty.NullVal(cty.String),
											"deferred": cty.True,
										})),
										After: mustPlanDynamicValue(cty.NullVal(cty.String)),
									},
								},
								PriorStateSrc: &states.ResourceInstanceObjectSrc{
									AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
										"id":       "deferred",
										"value":    nil,
										"deferred": true,
									}),
									Status:       states.ObjectReady,
									Dependencies: make([]addrs.ConfigResource, 0),
								},
								ProviderConfigAddr: mustDefaultRootProvider("testing"),
								Schema:             stacks_testing_provider.DeferredResourceSchema,
							},
							DeferredReason: "resource_config_unknown",
						},
						&stackplan.PlannedChangeComponentInstance{
							Addr:          mustAbsComponentInstance("component.valid"),
							PlanApplyable: false,
							Action:        plans.Delete,
							Mode:          plans.DestroyMode,
							PlannedInputValues: map[string]plans.DynamicValue{
								"id":    mustPlanDynamicValueDynamicType(cty.StringVal("valid")),
								"input": mustPlanDynamicValueDynamicType(cty.StringVal("valid")),
							},
							PlannedInputValueMarks: map[string][]cty.PathValueMarks{
								"id":    nil,
								"input": nil,
							},
							PlannedOutputValues: make(map[string]cty.Value),
							PlannedCheckResults: &states.CheckResults{},
							PlanTimestamp:       fakePlanTimestamp,
						},
						&stackplan.PlannedChangeDeferredResourceInstancePlanned{
							ResourceInstancePlanned: stackplan.PlannedChangeResourceInstancePlanned{
								ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.valid.testing_resource.data"),
								ChangeSrc: &plans.ResourceInstanceChangeSrc{
									Addr:         mustAbsResourceInstance("testing_resource.data"),
									PrevRunAddr:  mustAbsResourceInstance("testing_resource.data"),
									ProviderAddr: mustDefaultRootProvider("testing"),
									ChangeSrc: plans.ChangeSrc{
										Action: plans.Delete,
										Before: mustPlanDynamicValue(cty.ObjectVal(map[string]cty.Value{
											"id":    cty.StringVal("valid"),
											"value": cty.StringVal("valid"),
										})),
										After: mustPlanDynamicValue(cty.NullVal(cty.String)),
									},
								},
								PriorStateSrc: &states.ResourceInstanceObjectSrc{
									AttrsJSON: mustMarshalJSONAttrs(map[string]interface{}{
										"id":    "valid",
										"value": "valid",
									}),
									Status:       states.ObjectReady,
									Dependencies: make([]addrs.ConfigResource, 0),
								},
								ProviderConfigAddr: mustDefaultRootProvider("testing"),
								Schema:             stacks_testing_provider.TestingResourceSchema,
							},
							DeferredReason: "deferred_prereq",
						},
						&stackplan.PlannedChangeHeader{
							TerraformVersion: version.SemVer,
						},
						&stackplan.PlannedChangePlannedTimestamp{
							PlannedTimestamp: fakePlanTimestamp,
						},
					},
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.deferred"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.deferred"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.valid")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("deferred"),
								mustInputVariable("defer"): cty.True,
							},
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.valid"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.valid"),
							Dependents:            collections.NewSet(mustAbsComponent("component.deferred")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("valid"),
								mustInputVariable("input"): cty.StringVal("valid"),
							},
						},
					},
				},
			},
		},
		"destroy-with-input-dependency": {
			path:        path.Join("with-single-input-and-output", "input-dependency"),
			description: "tests destroy operations with input dependencies",
			cycles: []TestCycle{
				{
					// Just create everything normally, and don't validate it.
					planMode: plans.NormalMode,
				},
				{
					planMode: plans.DestroyMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.child"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.child"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.parent")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("child"),
								mustInputVariable("input"): cty.StringVal("parent"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.child.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.parent"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.parent"),
							Dependents:            collections.NewSet(mustAbsComponent("component.child")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("parent"),
								mustInputVariable("input"): cty.StringVal("parent"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.parent.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
					},
				},
			},
		},
		"destroy-with-provider-dependency": {
			path:        path.Join("with-single-input-and-output", "provider-dependency"),
			description: "tests destroy operations with provider dependencies",
			cycles: []TestCycle{
				{
					// Just create everything normally, and don't validate it.
					planMode: plans.NormalMode,
				},
				{
					planMode: plans.DestroyMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.child"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.child"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.parent")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("child"),
								mustInputVariable("input"): cty.StringVal("child"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.child.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.parent"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.parent"),
							Dependents:            collections.NewSet(mustAbsComponent("component.child")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("parent"),
								mustInputVariable("input"): cty.StringVal("parent"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.parent.testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
					},
				},
			},
		},
		"destroy-with-for-each-dependency": {
			path:        path.Join("with-single-input-and-output", "for-each-dependency"),
			description: "tests destroy operations with for-each dependencies",
			cycles: []TestCycle{
				{
					// Just create everything normally, and don't validate it.
					planMode: plans.NormalMode,
				},
				{
					planMode: plans.DestroyMode,
					wantAppliedChanges: []stackstate.AppliedChange{
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.child"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.child[\"a\"]"),
							Dependencies:          collections.NewSet(mustAbsComponent("component.parent")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("child:a"),
								mustInputVariable("input"): cty.StringVal("child"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.child[\"a\"].testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
						&stackstate.AppliedChangeComponentInstance{
							ComponentAddr:         mustAbsComponent("component.parent"),
							ComponentInstanceAddr: mustAbsComponentInstance("component.parent[\"a\"]"),
							Dependents:            collections.NewSet(mustAbsComponent("component.child")),
							OutputValues:          make(map[addrs.OutputValue]cty.Value),
							InputVariables: map[addrs.InputVariable]cty.Value{
								mustInputVariable("id"):    cty.StringVal("a"),
								mustInputVariable("input"): cty.StringVal("parent"),
							},
						},
						&stackstate.AppliedChangeResourceInstanceObject{
							ResourceInstanceObjectAddr: mustAbsResourceInstanceObject("component.parent[\"a\"].testing_resource.data"),
							ProviderConfigAddr:         mustDefaultRootProvider("testing"),
						},
					},
				},
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			lock := depsfile.NewLocks()
			lock.SetProvider(
				addrs.NewDefaultProvider("testing"),
				providerreqs.MustParseVersion("0.0.0"),
				providerreqs.MustParseVersionConstraints("=0.0.0"),
				providerreqs.PreferredHashes([]providerreqs.Hash{}),
			)

			store := tc.store
			if store == nil {
				store = stacks_testing_provider.NewResourceStore()
			}

			testContext := TestContext{
				timestamp: &fakePlanTimestamp,
				config:    loadMainBundleConfigForTest(t, tc.path),
				providers: map[addrs.Provider]providers.Factory{
					addrs.NewDefaultProvider("testing"): func() (providers.Interface, error) {
						return stacks_testing_provider.NewProviderWithData(t, store), nil
					},
				},
				dependencyLocks: *lock,
			}

			state := tc.state
			for ix, cycle := range tc.cycles {
				t.Run(strconv.FormatInt(int64(ix), 10), func(t *testing.T) {
					var plan *stackplan.Plan
					t.Run("plan", func(t *testing.T) {
						plan = testContext.Plan(t, ctx, state, cycle)
					})
					t.Run("apply", func(t *testing.T) {
						state = testContext.Apply(t, ctx, plan, cycle)
					})
				})
			}

		})
	}
}
