//go:build !noforge

package main

import "hubnot/internal/forge"

func init() {
	// The legacy issue-only protocol cannot express process mappings, an explicit
	// API profile or credential mode. Advertise that boundary before network I/O;
	// the project migration registry implements mapped Work Item/comment CRUD.
	capabilities := []forge.Capability{}
	for _, resource := range forge.Resources() {
		capabilities = append(capabilities, forge.Capability{Resource: resource, Reason: "Azure requires mapped project migration with explicit organization/project, API/auth profile, process/type/state mappings; legacy issue transfer cannot express these inputs", MappedFields: []string{}, Unsupported: []string{"legacy issue/open-closed mapping", "implicit API/auth/process defaults"}})
	}
	forgeProviders["azure"] = forgeRegistration{Capabilities: capabilities}
}
