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

package k8sutil

import (
	"encoding/json"
	"fmt"

	api "github.com/nuance-mobility/zookeeper-operator/pkg/apis/zookeeper/v1alpha1"
	//"github.com/nuance-mobility/zookeeper-operator/pkg/util/zookeeperutil"

	"k8s.io/api/core/v1"
)

const (
	zookeeperDataVolumeName = "zookeeper-data"
	zookeeperTlogVolumeName = "zookeeper-tlog"
)

func zookeeperVolumeMounts() []v1.VolumeMount {
	return []v1.VolumeMount{
		{Name: zookeeperDataVolumeName, MountPath: zookeeperDataVolumeMountDir},
		{Name: zookeeperTlogVolumeName, MountPath: zookeeperTlogVolumeMountDir},
	}
}

func zookeeperContainer(repo, version string) v1.Container {
	c := v1.Container{
		Name:    "zookeeper",
		Image:   ImageName(repo, version),
		Ports: []v1.ContainerPort{
			{
				Name:          "client",
				ContainerPort: int32(ZookeeperClientPort),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "peer",
				ContainerPort: int32(2888),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "server",
				ContainerPort: int32(3888),
				Protocol:      v1.ProtocolTCP,
			},
			{
				Name:          "jolokia",
				ContainerPort: int32(8778),
				Protocol:      v1.ProtocolTCP,
			},
		},
		VolumeMounts: zookeeperVolumeMounts(),
	}

	return c
}

func containerWithProbes(c v1.Container, lp *v1.Probe, rp *v1.Probe) v1.Container {
	c.LivenessProbe = lp
	c.ReadinessProbe = rp
	return c
}

func containerWithRequirements(c v1.Container, r v1.ResourceRequirements) v1.Container {
	c.Resources = r
	return c
}

func newZookeeperProbe() *v1.Probe {
	cmd := fmt.Sprintf("zkOk.sh %d", ZookeeperClientPort)
	return &v1.Probe{
		Handler: v1.Handler{
			Exec: &v1.ExecAction{
				Command: []string{"/bin/sh", "-c", cmd},
			},
		},
		InitialDelaySeconds: 10,
		TimeoutSeconds:      10,
		PeriodSeconds:       60,
		FailureThreshold:    3,
	}
}

func applyPodPolicy(clusterName string, pod *v1.Pod, policy *api.PodPolicy) {
	if policy == nil {
		return
	}

	if policy.Affinity != nil {
		pod.Spec.Affinity = policy.Affinity
	}

	if len(policy.NodeSelector) != 0 {
		pod = PodWithNodeSelector(pod, policy.NodeSelector)
	}
	if len(policy.Tolerations) != 0 {
		pod.Spec.Tolerations = policy.Tolerations
	}

	mergeLabels(pod.Labels, policy.Labels)

	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i] = containerWithRequirements(pod.Spec.Containers[i], policy.Resources)
		if pod.Spec.Containers[i].Name == "zookeeper" {
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, policy.ZookeeperEnv...)
		}
	}

	for i := range pod.Spec.InitContainers {
		pod.Spec.InitContainers[i] = containerWithRequirements(pod.Spec.InitContainers[i], policy.Resources)
	}

	for key, value := range policy.Annotations {
		pod.ObjectMeta.Annotations[key] = value
	}
}

// IsPodReady returns false if the Pod Status is nil
func IsPodReady(pod *v1.Pod) bool {
	condition := getPodReadyCondition(&pod.Status)
	return condition != nil && condition.Status == v1.ConditionTrue
}

func getPodReadyCondition(status *v1.PodStatus) *v1.PodCondition {
	for i := range status.Conditions {
		if status.Conditions[i].Type == v1.PodReady {
			return &status.Conditions[i]
		}
	}
	return nil
}

func PodSpecToPrettyJSON(pod *v1.Pod) (string, error) {
	bytes, err := json.MarshalIndent(pod.Spec, "", "    ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
