// Copyright 2022 Butler Labs Labs
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"fmt"
	"runtime"

	"github.com/pkg/errors"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"

	stewardv1alpha1 "github.com/butlerdotdev/steward/api/v1alpha1"
)

type stewardKubeVersionGetter struct {
	upgrade.VersionGetter

	k8sVersion     string
	coreDNSVersion string
	status         *stewardv1alpha1.KubernetesVersionStatus
}

func NewStewardKubeVersionGetter(restClient kubernetes.Interface, version, coreDNSVersion string, status *stewardv1alpha1.KubernetesVersionStatus) upgrade.VersionGetter {
	kubeVersionGetter := upgrade.NewOfflineVersionGetter(upgrade.NewKubeVersionGetter(restClient), KubeadmVersion)

	return &stewardKubeVersionGetter{
		VersionGetter:  kubeVersionGetter,
		k8sVersion:     version,
		coreDNSVersion: coreDNSVersion,
		status:         status,
	}
}

func (k stewardKubeVersionGetter) ClusterVersion() (string, *versionutil.Version, error) {
	if k.status != nil && *k.status == stewardv1alpha1.VersionSleeping {
		parsedVersion, parsedErr := versionutil.ParseGeneric(k.k8sVersion)

		return k.k8sVersion, parsedVersion, parsedErr
	}

	return k.VersionGetter.ClusterVersion()
}

func (k stewardKubeVersionGetter) DNSAddonVersion() (string, error) {
	if k.status != nil && *k.status == stewardv1alpha1.VersionSleeping {
		return k.coreDNSVersion, nil
	}

	return k.VersionGetter.DNSAddonVersion()
}

func (k stewardKubeVersionGetter) KubeadmVersion() (string, *versionutil.Version, error) {
	kubeadmVersionInfo := apimachineryversion.Info{
		GitVersion: KubeadmVersion,
		GoVersion:  runtime.Version(),
		Compiler:   runtime.Compiler,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	kubeadmVersion, err := versionutil.ParseSemantic(kubeadmVersionInfo.String())
	if err != nil {
		return "", nil, errors.Wrap(err, "Couldn't parse kubeadm version")
	}

	return kubeadmVersionInfo.String(), kubeadmVersion, nil
}

func (k stewardKubeVersionGetter) VersionFromCILabel(ciVersionLabel, description string) (string, *versionutil.Version, error) {
	return k.VersionGetter.VersionFromCILabel(ciVersionLabel, description)
}

func (k stewardKubeVersionGetter) KubeletVersions() (map[string][]string, error) {
	if k.status != nil && *k.status == stewardv1alpha1.VersionSleeping {
		return map[string][]string{}, nil
	}

	return k.VersionGetter.KubeletVersions()
}

func (k stewardKubeVersionGetter) ComponentVersions(string) (map[string][]string, error) {
	return map[string][]string{
		k.k8sVersion: {"steward"},
	}, nil
}
