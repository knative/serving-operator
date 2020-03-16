// +build !ignore_autogenerated

/*
Copyright 2020 The Knative Authors

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	apis "knative.dev/pkg/apis"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomCerts) DeepCopyInto(out *CustomCerts) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CustomCerts.
func (in *CustomCerts) DeepCopy() *CustomCerts {
	if in == nil {
		return nil
	}
	out := new(CustomCerts)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HighAvailability) DeepCopyInto(out *HighAvailability) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HighAvailability.
func (in *HighAvailability) DeepCopy() *HighAvailability {
	if in == nil {
		return nil
	}
	out := new(HighAvailability)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IstioGatewayOverride) DeepCopyInto(out *IstioGatewayOverride) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IstioGatewayOverride.
func (in *IstioGatewayOverride) DeepCopy() *IstioGatewayOverride {
	if in == nil {
		return nil
	}
	out := new(IstioGatewayOverride)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KnativeServing) DeepCopyInto(out *KnativeServing) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KnativeServing.
func (in *KnativeServing) DeepCopy() *KnativeServing {
	if in == nil {
		return nil
	}
	out := new(KnativeServing)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KnativeServing) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KnativeServingList) DeepCopyInto(out *KnativeServingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KnativeServing, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KnativeServingList.
func (in *KnativeServingList) DeepCopy() *KnativeServingList {
	if in == nil {
		return nil
	}
	out := new(KnativeServingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KnativeServingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KnativeServingSpec) DeepCopyInto(out *KnativeServingSpec) {
	*out = *in
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = make(map[string]map[string]string, len(*in))
		for key, val := range *in {
			var outVal map[string]string
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make(map[string]string, len(*in))
				for key, val := range *in {
					(*out)[key] = val
				}
			}
			(*out)[key] = outVal
		}
	}
	in.Registry.DeepCopyInto(&out.Registry)
	in.KnativeIngressGateway.DeepCopyInto(&out.KnativeIngressGateway)
	in.ClusterLocalGateway.DeepCopyInto(&out.ClusterLocalGateway)
	out.ControllerCustomCerts = in.ControllerCustomCerts
	if in.HighAvailability != nil {
		in, out := &in.HighAvailability, &out.HighAvailability
		*out = new(HighAvailability)
		**out = **in
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]ResourceRequirementsOverride, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KnativeServingSpec.
func (in *KnativeServingSpec) DeepCopy() *KnativeServingSpec {
	if in == nil {
		return nil
	}
	out := new(KnativeServingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KnativeServingStatus) DeepCopyInto(out *KnativeServingStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(apis.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KnativeServingStatus.
func (in *KnativeServingStatus) DeepCopy() *KnativeServingStatus {
	if in == nil {
		return nil
	}
	out := new(KnativeServingStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Registry) DeepCopyInto(out *Registry) {
	*out = *in
	if in.Override != nil {
		in, out := &in.Override, &out.Override
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]v1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Registry.
func (in *Registry) DeepCopy() *Registry {
	if in == nil {
		return nil
	}
	out := new(Registry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceRequirementsOverride) DeepCopyInto(out *ResourceRequirementsOverride) {
	*out = *in
	in.ResourceRequirements.DeepCopyInto(&out.ResourceRequirements)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceRequirementsOverride.
func (in *ResourceRequirementsOverride) DeepCopy() *ResourceRequirementsOverride {
	if in == nil {
		return nil
	}
	out := new(ResourceRequirementsOverride)
	in.DeepCopyInto(out)
	return out
}
