package v1

import (
	"errors"

	"github.com/appscode/go/types"
	"github.com/appscode/kutil/meta"
	"github.com/appscode/mergo"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetGroupVersionKind(v interface{}) schema.GroupVersionKind {
	return core.SchemeGroupVersion.WithKind(meta.GetKind(v))
}

func AssignTypeKind(v interface{}) error {
	_, err := conversion.EnforcePtr(v)
	if err != nil {
		return err
	}

	switch u := v.(type) {
	case *core.Pod:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.ReplicationController:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.ConfigMap:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.Secret:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.Service:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.PersistentVolumeClaim:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.PersistentVolume:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.Node:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.ServiceAccount:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.Namespace:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.Endpoints:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.ComponentStatus:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.LimitRange:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	case *core.Event:
		u.APIVersion = core.SchemeGroupVersion.String()
		u.Kind = meta.GetKind(v)
		return nil
	}
	return errors.New("unknown api object type")
}

func RemoveNextInitializer(m metav1.ObjectMeta) metav1.ObjectMeta {
	if m.GetInitializers() != nil {
		pendingInitializers := m.GetInitializers().Pending
		// Remove self from the list of pending Initializers while preserving ordering.
		if len(pendingInitializers) == 1 {
			m.Initializers = nil
		} else {
			m.Initializers.Pending = append(pendingInitializers[:0], pendingInitializers[1:]...)
		}
	}
	return m
}

func AddFinalizer(m metav1.ObjectMeta, finalizer string) metav1.ObjectMeta {
	for _, name := range m.Finalizers {
		if name == finalizer {
			return m
		}
	}
	m.Finalizers = append(m.Finalizers, finalizer)
	return m
}

func HasFinalizer(m metav1.ObjectMeta, finalizer string) bool {
	for _, name := range m.Finalizers {
		if name == finalizer {
			return true
		}
	}
	return false
}

func RemoveFinalizer(m metav1.ObjectMeta, finalizer string) metav1.ObjectMeta {
	// https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	r := m.Finalizers[:0]
	for _, name := range m.Finalizers {
		if name != finalizer {
			r = append(r, name)
		}
	}
	m.Finalizers = r
	return m
}

func EnsureContainerDeleted(containers []core.Container, name string) []core.Container {
	for i, c := range containers {
		if c.Name == name {
			return append(containers[:i], containers[i+1:]...)
		}
	}
	return containers
}

func UpsertContainer(containers []core.Container, upsert core.Container) []core.Container {
	for i, container := range containers {
		if container.Name == upsert.Name {
			err := mergo.MergeWithOverwrite(&container, upsert)
			if err != nil {
				panic(err)
			}
			containers[i] = container
			return containers
		}
	}
	return append(containers, upsert)
}

func UpsertVolume(volumes []core.Volume, nv core.Volume) []core.Volume {
	for i, vol := range volumes {
		if vol.Name == nv.Name {
			volumes[i] = nv
			return volumes
		}
	}
	return append(volumes, nv)
}

func UpsertVolumeClaim(volumeClaims []core.PersistentVolumeClaim, upsert core.PersistentVolumeClaim) []core.PersistentVolumeClaim {
	for i, vc := range volumeClaims {
		if vc.Name == upsert.Name {
			volumeClaims[i] = upsert
			return volumeClaims
		}
	}
	return append(volumeClaims, upsert)
}

func EnsureVolumeDeleted(volumes []core.Volume, name string) []core.Volume {
	for i, v := range volumes {
		if v.Name == name {
			return append(volumes[:i], volumes[i+1:]...)
		}
	}
	return volumes
}

func UpsertVolumeMount(mounts []core.VolumeMount, nv core.VolumeMount) []core.VolumeMount {
	for i, vol := range mounts {
		if vol.Name == nv.Name {
			mounts[i] = nv
			return mounts
		}
	}
	return append(mounts, nv)
}

func EnsureVolumeMountDeleted(mounts []core.VolumeMount, name string) []core.VolumeMount {
	for i, v := range mounts {
		if v.Name == name {
			return append(mounts[:i], mounts[i+1:]...)
		}
	}
	return mounts
}

func UpsertEnvVars(vars []core.EnvVar, nv ...core.EnvVar) []core.EnvVar {
	upsert := func(env core.EnvVar) {
		for i, v := range vars {
			if v.Name == env.Name {
				vars[i] = env
				return
			}
		}
		vars = append(vars, env)
	}

	for _, env := range nv {
		upsert(env)
	}
	return vars
}

func EnsureEnvVarDeleted(vars []core.EnvVar, name string) []core.EnvVar {
	for i, v := range vars {
		if v.Name == name {
			return append(vars[:i], vars[i+1:]...)
		}
	}
	return vars
}

func UpsertMap(maps, upsert map[string]string) map[string]string {
	if maps == nil {
		maps = make(map[string]string)
	}
	for k, v := range upsert {
		maps[k] = v
	}
	return maps
}

func MergeLocalObjectReferences(old, new []core.LocalObjectReference) []core.LocalObjectReference {
	m := make(map[string]core.LocalObjectReference)
	for _, ref := range old {
		m[ref.Name] = ref
	}
	for _, ref := range new {
		m[ref.Name] = ref
	}

	result := make([]core.LocalObjectReference, 0, len(m))
	for _, ref := range m {
		result = append(result, ref)
	}
	return result
}

func EnsureOwnerReference(meta metav1.ObjectMeta, owner *core.ObjectReference) metav1.ObjectMeta {
	fi := -1
	for i, ref := range meta.OwnerReferences {
		if ref.Kind == owner.Kind && ref.Name == owner.Name {
			fi = i
			break
		}
	}
	if fi == -1 {
		meta.OwnerReferences = append(meta.OwnerReferences, metav1.OwnerReference{})
		fi = len(meta.OwnerReferences) - 1
	}
	meta.OwnerReferences[fi].APIVersion = owner.APIVersion
	meta.OwnerReferences[fi].Kind = owner.Kind
	meta.OwnerReferences[fi].Name = owner.Name
	meta.OwnerReferences[fi].UID = owner.UID
	meta.OwnerReferences[fi].BlockOwnerDeletion = types.TrueP()
	return meta
}
