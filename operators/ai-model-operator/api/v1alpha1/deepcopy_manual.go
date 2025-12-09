package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyInto is a manual implementation of deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AIModelSpec) DeepCopyInto(out *AIModelSpec) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is a manual implementation of deepcopy function, copying the receiver, creating a new AIModelSpec.
func (in *AIModelSpec) DeepCopy() *AIModelSpec {
	if in == nil {
		return nil
	}
	out := new(AIModelSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a manual implementation of deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AIModelStatus) DeepCopyInto(out *AIModelStatus) {
	*out = *in
}

// DeepCopy is a manual implementation of deepcopy function, copying the receiver, creating a new AIModelStatus.
func (in *AIModelStatus) DeepCopy() *AIModelStatus {
	if in == nil {
		return nil
	}
	out := new(AIModelStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a manual implementation of deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AIModel) DeepCopyInto(out *AIModel) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is a manual implementation of deepcopy function, copying the receiver, creating a new AIModel.
func (in *AIModel) DeepCopy() *AIModel {
	if in == nil {
		return nil
	}
	out := new(AIModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is a manual implementation of deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AIModel) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is a manual implementation of deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AIModelList) DeepCopyInto(out *AIModelList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AIModel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is a manual implementation of deepcopy function, copying the receiver, creating a new AIModelList.
func (in *AIModelList) DeepCopy() *AIModelList {
	if in == nil {
		return nil
	}
	out := new(AIModelList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is a manual implementation of deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AIModelList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}