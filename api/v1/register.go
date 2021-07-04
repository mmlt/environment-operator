package v1

import "k8s.io/apimachinery/pkg/runtime/schema"

// This file provides some the values of groupversion_info.go for use by pkg/generated packages.

const GroupName = "clusterops.mmlt.nl"

var SchemeGroupVersion = schema.GroupVersion{Group: "clusterops.mmlt.nl", Version: "v1"}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
