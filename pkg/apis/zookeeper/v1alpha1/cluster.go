// Copyright 2018 The zookeeper-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"errors"
	"strings"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultRepository  = "blafrisch/zookeeper"
	DefaultZookeeperVersion = "3.5.3-beta"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ZookeeperClusterList is a list of zookeeper clusters.
type ZookeeperClusterList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZookeeperCluster `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ZookeeperCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClusterSpec   `json:"spec"`
	Status            ClusterStatus `json:"status"`
}

func (c *ZookeeperCluster) AsOwner() metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       ZookeeperClusterResourceKind,
		Name:       c.Name,
		UID:        c.UID,
		Controller: &trueVar,
	}
}

type JVMPolicy struct {
	HeapSizeInMB int `json:"heapSizeInMB"`

	NewGenSizeInMB int `json:"newGenSizeInMB"`

	TunuringThreshold int `json:"tenuringThreshold"`
}

type ClusterSpec struct {
	// Size is the expected size of the zookeeper cluster.
	// The zookeeper-operator will eventually make the size of the running
	// cluster equal to the expected size.
	// The valid range of the size is from 1 to infinite
	Size int `json:"size"`
	// Repository is the name of the repository that hosts
	// zookeeper container images. It should be direct clone of the repository in official
	// release:
	//   https://hub.docker.com/_/zookeeper/
	// That means, it should have exact same tags and the same meaning for the tags.
	//
	// By default, it is `zookeeper`.
	Repository string `json:"repository,omitempty"`

	// Version is the expected version of the zookeeper cluster.
	// The zookeeper-operator will eventually make the zookeeper cluster version
	// equal to the expected version.
	//
	// The version must follow the [semver]( http://semver.org) format, for example "3.5.3-beta".
	// Only zookeeper released versions are supported: https://hub.docker.com/blafrisch/zookeeper/
	//
	// If version is not set, default is "3.5.3-beta".
	Version string `json:"version,omitempty"`

	// Paused is to pause the control of the operator for the zookeeper cluster.
	Paused bool `json:"paused,omitempty"`

	// Pod defines the policy to create pod for the zookeeper pod.
	//
	// Updating Pod does not take effect on any existing zookeeper pods.
	Pod *PodPolicy `json:"pod,omitempty"`

	// zookeeper JVM policy
	JVM *JVMPolicy `json:"jvm,omitempty"`
}

// PodPolicy defines the policy to create pod for the zookeeper container.
type PodPolicy struct {
	// Labels specifies the labels to attach to pods the operator creates for the
	// zookeeper cluster.
	// "app" and "zookeeper_*" labels are reserved for the internal use of the zookeeper operator.
	// Do not overwrite them.
	Labels map[string]string `json:"labels,omitempty"`

	// NodeSelector specifies a map of key-value pairs. For the pod to be eligible
	// to run on a node, the node must have each of the indicated key-value pairs as
	// labels.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// The scheduling constraints on zookeeper pods.
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// **DEPRECATED**. Use Affinity instead.
	AntiAffinity bool `json:"antiAffinity,omitempty"`

	// Resources is the resource requirements for the zookeeper container.
	// This field cannot be updated once the cluster is created.
	Resources v1.ResourceRequirements `json:"resources,omitempty"`

	// Tolerations specifies the pod's tolerations.
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// List of environment variables to set in the zookeeper container.
	// This is used to configure zookeeper process. zookeeper cluster cannot be created, when
	// bad environement variables are provided. Do not overwrite any flags used to
	// bootstrap the cluster (for example `--initial-cluster` flag).
	// This field cannot be updated.
	ZookeeperEnv []v1.EnvVar `json:"zookeeperEnv,omitempty"`

	// PersistentVolumeClaimSpec is the spec to describe PVC for the zookeeper container
	// This field is optional. If no PVC spec, zookeeper container will use emptyDir as volume
	// Note. This feature is in alpha stage. It is currently only used as non-stable storage,
	// not the stable storage. Future work need to make it used as stable storage.
	PersistentVolumeClaimSpec *v1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"`

	// Annotations specifies the annotations to attach to pods the operator creates for the
	// zookeeper cluster.
	// The "zookeeper.version" annotation is reserved for the internal use of the zookeeper operator.
	Annotations map[string]string `json:"annotations,omitempty"`

	// busybox init container image. default is busybox:1.28.0-glibc
	// busybox:latest uses uclibc which contains a bug that sometimes prevents name resolution
	// More info: https://github.com/docker-library/busybox/issues/27
	BusyboxImage string `json:"busyboxImage,omitempty"`
}

// TODO: move this to initializer
func (c *ClusterSpec) Validate() error {
	/*
	if c.TLS != nil {
		if err := c.TLS.Validate(); err != nil {
			return err
		}
	}
	*/

	if c.Pod != nil {
		for k := range c.Pod.Labels {
			if k == "app" || strings.HasPrefix(k, "zookeeper_") {
				return errors.New("spec: pod labels contains reserved label")
			}
		}
	}
	return nil
}

// SetDefaults cleans up user passed spec, e.g. defaulting, transforming fields.
// TODO: move this to initializer
func (e *ZookeeperCluster) SetDefaults() {
	c := &e.Spec
	if len(c.Repository) == 0 {
		c.Repository = defaultRepository
	}

	if len(c.Version) == 0 {
		c.Version = DefaultZookeeperVersion
	}

	c.Version = strings.TrimLeft(c.Version, "v")

	// convert PodPolicy.AntiAffinity to Pod.Affinity.PodAntiAffinity
	// TODO: Remove this once PodPolicy.AntiAffinity is removed
	if c.Pod != nil && c.Pod.AntiAffinity && c.Pod.Affinity == nil {
		c.Pod.Affinity = &v1.Affinity{
			PodAntiAffinity: &v1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
					{
						// set anti-affinity to the zookeeper pods that belongs to the same cluster
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
							"zookeeper_cluster": e.Name,
						}},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		}
	}
}
