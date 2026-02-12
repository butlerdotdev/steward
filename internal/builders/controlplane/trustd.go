// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	pointer "k8s.io/utils/ptr"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
	"github.com/butlerdotdev/steward/internal/utilities"
)

const (
	trustdContainerName = "steward-trustd"
	trustdCertsVolume   = "trustd-creds"
	trustdCertsPath     = "/etc/steward-trustd/certs"
)

type Trustd struct {
	Scheme runtime.Scheme
}

func (t Trustd) Build(deployment *appsv1.Deployment, tcp stewardv1alpha1.TenantControlPlane) {
	t.buildContainer(tcp, &deployment.Spec.Template.Spec)
	t.buildVolumes(tcp, &deployment.Spec.Template.Spec)
	t.Scheme.Default(deployment)
}

func (t Trustd) buildContainer(tcp stewardv1alpha1.TenantControlPlane, podSpec *corev1.PodSpec) {
	spec := tcp.Spec.Addons.WorkerBootstrap.Talos

	found, index := utilities.HasNamedContainer(podSpec.Containers, trustdContainerName)
	if !found {
		index = len(podSpec.Containers)
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	image := spec.Image
	if spec.ImageTag != "" {
		image = fmt.Sprintf("%s:%s", spec.Image, spec.ImageTag)
	}

	podSpec.Containers[index].Name = trustdContainerName
	podSpec.Containers[index].Image = image
	podSpec.Containers[index].Ports = []corev1.ContainerPort{
		{
			Name:          "grpc",
			ContainerPort: spec.Port,
			Protocol:      corev1.ProtocolTCP,
		},
	}
	podSpec.Containers[index].Env = []corev1.EnvVar{
		{
			Name:  "TRUSTD_PORT",
			Value: fmt.Sprintf("%d", spec.Port),
		},
		{
			Name:  "TRUSTD_TENANT",
			Value: tcp.GetName(),
		},
		{
			Name:  "TRUSTD_CERT_DIR",
			Value: trustdCertsPath,
		},
		{
			Name: "TRUSTD_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-trustd-creds", tcp.GetName()),
					},
					Key: "token",
				},
			},
		},
	}
	podSpec.Containers[index].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      trustdCertsVolume,
			MountPath: trustdCertsPath,
			ReadOnly:  true,
		},
	}
	podSpec.Containers[index].ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(spec.Port),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
	}
	podSpec.Containers[index].LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(spec.Port),
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       20,
	}
	podSpec.Containers[index].SecurityContext = &corev1.SecurityContext{
		RunAsNonRoot:             pointer.To(true),
		RunAsUser:                pointer.To(int64(65534)),
		ReadOnlyRootFilesystem:   pointer.To(true),
		AllowPrivilegeEscalation: pointer.To(false),
	}

	if spec.Resources != nil {
		podSpec.Containers[index].Resources = *spec.Resources
	} else {
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		}
	}
}

func (t Trustd) buildVolumes(tcp stewardv1alpha1.TenantControlPlane, podSpec *corev1.PodSpec) {
	found, index := utilities.HasNamedVolume(podSpec.Volumes, trustdCertsVolume)
	if !found {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
		index = len(podSpec.Volumes) - 1
	}

	podSpec.Volumes[index].Name = trustdCertsVolume
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  fmt.Sprintf("%s-trustd-creds", tcp.GetName()),
			DefaultMode: pointer.To(int32(420)),
		},
	}
}

// RemoveContainer removes the steward-trustd container from the pod spec.
func (t Trustd) RemoveContainer(podSpec *corev1.PodSpec) {
	if found, index := utilities.HasNamedContainer(podSpec.Containers, trustdContainerName); found {
		podSpec.Containers = append(podSpec.Containers[:index], podSpec.Containers[index+1:]...)
	}
}

// RemoveVolumes removes the steward-trustd volumes from the pod spec.
func (t Trustd) RemoveVolumes(podSpec *corev1.PodSpec) {
	if found, index := utilities.HasNamedVolume(podSpec.Volumes, trustdCertsVolume); found {
		podSpec.Volumes = append(podSpec.Volumes[:index], podSpec.Volumes[index+1:]...)
	}
}
