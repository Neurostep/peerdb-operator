/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1alpha1 contains API Schema definitions for the peerdb v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=peerdb.peerdb.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "peerdb.peerdb.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Builder registers go types with a Scheme for a single GroupVersion. It
// replaces the deprecated sigs.k8s.io/controller-runtime/pkg/scheme.Builder so
// the api package stays free of controller-runtime dependencies.
//
// +kubebuilder:object:generate=false
type Builder struct {
	GroupVersion schema.GroupVersion
	runtime.SchemeBuilder
}

// Register queues one or more objects for registration under the Builder's
// GroupVersion when AddToScheme is called.
func (bld *Builder) Register(objects ...runtime.Object) *Builder {
	bld.SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(bld.GroupVersion, objects...)
		metav1.AddToGroupVersion(s, bld.GroupVersion)
		return nil
	})
	return bld
}
