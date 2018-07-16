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
	"net"
	"os"
	"strings"
	"strconv"
	"time"

	api "github.com/nuance-mobility/zookeeper-operator/pkg/apis/zookeeper/v1alpha1"
	"github.com/nuance-mobility/zookeeper-operator/pkg/util/zookeeperutil"
	"github.com/nuance-mobility/zookeeper-operator/pkg/util/retryutil"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // for gcp auth
	"k8s.io/client-go/rest"
)

const (
	// ZookeeperClientPort is the client port on client service and zookeeper nodes.
	ZookeeperClientPort = 2181

	zookeeperDataVolumeMountDir = "/data"
	zookeeperTlogVolumeMountDir = "/datalog"
	zookeeperVersionAnnotationKey = "zookeeper.version"

	randomSuffixLength = 10
	// k8s object name has a maximum length
	maxNameLength = 63 - randomSuffixLength - 1

	defaultBusyboxImage = "busybox:1.28.0-glibc"

	defaultKubeAPIRequestTimeout = 30 * time.Second

	// AnnotationScope annotation name for defining instance scope. Used for specifing cluster wide clusters.
	AnnotationScope = "zookeeper.database.apache.com/scope"
	//AnnotationClusterWide annotation value for cluster wide clusters.
	AnnotationClusterWide = "clusterwide"

	DefaultCPU = ".1"
	DefaultMEM = "512m"
)

const TolerateUnreadyEndpointsAnnotation = "service.alpha.kubernetes.io/tolerate-unready-endpoints"

func GetZookeeperVersion(pod *v1.Pod) string {
	return pod.Annotations[zookeeperVersionAnnotationKey]
}

func SetZookeeperVersion(pod *v1.Pod, version string) {
	pod.Annotations[zookeeperVersionAnnotationKey] = version
}

func GetPodNames(pods []*v1.Pod) []string {
	if len(pods) == 0 {
		return nil
	}
	res := []string{}
	for _, p := range pods {
		res = append(res, p.Name)
	}
	return res
}

// PVCNameFromMember the way we get PVC name from the member name
func PVCNameFromMember(memberName string) string {
	return memberName
}

func ImageName(repo, version string) string {
	return fmt.Sprintf("%s:v%v", repo, version)
}

// imageNameBusybox returns the default image for busybox init container, or the image specified in the PodPolicy
func imageNameBusybox(policy *api.PodPolicy) string {
	if policy != nil && len(policy.BusyboxImage) > 0 {
		return policy.BusyboxImage
	}
	return defaultBusyboxImage
}

func PodWithNodeSelector(p *v1.Pod, ns map[string]string) *v1.Pod {
	p.Spec.NodeSelector = ns
	return p
}

func CreateClientService(kubecli kubernetes.Interface, clusterName, ns string, owner metav1.OwnerReference) error {
	ports := []v1.ServicePort{{
		Name:       "client",
		Port:       ZookeeperClientPort,
		TargetPort: intstr.FromInt(ZookeeperClientPort),
		Protocol:   v1.ProtocolTCP,
	}}
	return createService(kubecli, ClientServiceName(clusterName), clusterName, ns, "", ports, owner)
}

func ClientServiceName(clusterName string) string {
	return clusterName + "-client"
}

func CreatePeerService(kubecli kubernetes.Interface, clusterName, ns string, owner metav1.OwnerReference) error {
	ports := []v1.ServicePort{{
		Name:       "client",
		Port:       ZookeeperClientPort,
		TargetPort: intstr.FromInt(ZookeeperClientPort),
		Protocol:   v1.ProtocolTCP,
	}, {
		Name:       "peer",
		Port:       2888,
		TargetPort: intstr.FromInt(2888),
		Protocol:   v1.ProtocolTCP,
	}, {
		Name:       "leader",
		Port:       3888,
		TargetPort: intstr.FromInt(3888),
		Protocol:   v1.ProtocolTCP,
	}}

	return createService(kubecli, clusterName, clusterName, ns, v1.ClusterIPNone, ports, owner)
}

func createService(kubecli kubernetes.Interface, svcName, clusterName, ns, clusterIP string, ports []v1.ServicePort, owner metav1.OwnerReference) error {
	svc := newZookeeperServiceManifest(svcName, clusterName, clusterIP, ports)
	addOwnerRefToObject(svc.GetObjectMeta(), owner)
	_, err := kubecli.CoreV1().Services(ns).Create(svc)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// CreateAndWaitPod creates a pod and waits until it is running
func CreateAndWaitPod(kubecli kubernetes.Interface, ns string, pod *v1.Pod, timeout time.Duration) (*v1.Pod, error) {
	_, err := kubecli.CoreV1().Pods(ns).Create(pod)
	if err != nil {
		return nil, err
	}

	interval := 5 * time.Second
	var retPod *v1.Pod
	err = retryutil.Retry(interval, int(timeout/(interval)), func() (bool, error) {
		retPod, err = kubecli.CoreV1().Pods(ns).Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch retPod.Status.Phase {
		case v1.PodRunning:
			return true, nil
		case v1.PodPending:
			return false, nil
		default:
			return false, fmt.Errorf("unexpected pod status.phase: %v", retPod.Status.Phase)
		}
	})

	if err != nil {
		if retryutil.IsRetryFailure(err) {
			return nil, fmt.Errorf("failed to wait pod running, it is still pending: %v", err)
		}
		return nil, fmt.Errorf("failed to wait pod running: %v", err)
	}

	return retPod, nil
}

func newZookeeperServiceManifest(svcName, clusterName, clusterIP string, ports []v1.ServicePort) *v1.Service {
	labels := LabelsForCluster(clusterName)
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   svcName,
			Labels: labels,
			Annotations: map[string]string{
				TolerateUnreadyEndpointsAnnotation: "true",
			},
		},
		Spec: v1.ServiceSpec{
			Ports:     ports,
			Selector:  labels,
			ClusterIP: clusterIP,
		},
	}
	return svc
}

// AddZookeeperVolumeToPod abstract the process of appending volume spec to pod spec
func AddZookeeperVolumeToPod(pod *v1.Pod, pvc *v1.PersistentVolumeClaim) {
	vol := v1.Volume{Name: zookeeperDataVolumeName}
	if pvc != nil {
		vol.VolumeSource = v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name},
		}
	} else {
		vol.VolumeSource = v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, vol)
}

func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
}

func NewZookeeperPod(m *zookeeperutil.Member, existingCluster []string, clusterName, state string, cs api.ClusterSpec, owner metav1.OwnerReference) *v1.Pod {
	labels := map[string]string{
		"app":          "zookeeper",
		"zookeeper_node":    m.Name,
		"zookeeper_cluster": clusterName,
	}

	livenessProbe := newZookeeperProbe()
	readinessProbe := newZookeeperProbe()
	readinessProbe.InitialDelaySeconds = 1
	readinessProbe.TimeoutSeconds = 5
	readinessProbe.PeriodSeconds = 5
	readinessProbe.FailureThreshold = 3

	container := containerWithProbes(
		zookeeperContainer(cs.Repository, cs.Version),
		livenessProbe,
		readinessProbe)

	zooServers := make([]string, len(existingCluster)+1)
	copy(zooServers, existingCluster)
	if state == "seed" || state == "replacement" {
		zooServers[len(existingCluster)] = fmt.Sprintf("server.%d=%s:2888:3888:participant;%s:2181", m.ID(), m.Addr(), m.Addr())
	} else {
		zooServers[len(existingCluster)] = fmt.Sprintf("server.%d=%s:2888:3888:observer;%s:2181", m.ID(), m.Addr(), m.Addr())
	}

	cpus, err := resource.ParseQuantity(cs.RequestCPU)
	if err != nil {
		cpus, _ = resource.ParseQuantity(DefaultCPU)
	}

	memory, err := resource.ParseQuantity(cs.RequestMEM)
	if err != nil {
		memory,_ = resource.ParseQuantity(DefaultMEM)
	}

	container.Resources= v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    cpus,
			v1.ResourceMemory: memory,
	}}

	container.Env = append(container.Env, v1.EnvVar{
		Name:  "ZOO_MY_ID",
		Value: strconv.Itoa(m.ID()),
	}, v1.EnvVar{
		Name:  "ZOO_SERVERS",
		Value: strings.Join(zooServers, " "),
	}, v1.EnvVar{
		Name:  "ZOO_MAX_CLIENT_CNXNS",
		Value: "0", // default 60
	})
	// Other available config items:
	// - ZOO_TICK_TIME: 2000
	// - ZOO_INIT_LIMIT: 5
	// - ZOO_SYNC_LIMIT: 2
	// - ZOO_STANDALONE_ENABLED: false (don't change this or you'll have a bad time)
	// - ZOO_RECONFIG_ENABLED: true (don't change this or you'll have a bad time)
	// - ZOO_SKIP_ACL: true
	// - ZOO_4LW_WHITELIST: ruok (probes will fail if ruok is removed)

	volumes := []v1.Volume{
		{Name: "zookeeper-data", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
		{Name: "zookeeper-tlog", VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}},
	}

	runAsNonRoot := true
	podUID := int64(1000)
	fsGroup := podUID
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        m.Name,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{{
				// busybox:latest uses uclibc which contains a bug that sometimes prevents name resolution
				// More info: https://github.com/docker-library/busybox/issues/27
				//Image default: "busybox:1.28.0-glibc",
				Image: imageNameBusybox(cs.Pod),
				Name:  "check-dns",
				// We bind to [hostname].[clustername].[namespace].svc which may take some time to appear in kubedns
				Command: []string{"/bin/sh", "-c", fmt.Sprintf(`
					while ( ! nslookup %s )
					do
						sleep 2
					done`, m.Addr())},
			}},
			Containers:    []v1.Container{container},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes:       volumes,
			// DNS A record: `[m.Name].[clusterName].Namespace.svc`
			// For example, zookeeper-795649v9kq in default namespace will have DNS name
			// `zookeeper-795649v9kq.zookeeper.default.svc`.
			Hostname:                     m.Name,
			Subdomain:                    clusterName,
			AutomountServiceAccountToken: func(b bool) *bool { return &b }(false),
			SecurityContext: &v1.PodSecurityContext{
				RunAsUser:    &podUID,
				RunAsNonRoot: &runAsNonRoot,
				FSGroup:      &fsGroup,
			},
		},
	}
	SetZookeeperVersion(pod, cs.Version)
	applyPodPolicy(clusterName, pod, cs.Pod)
	addOwnerRefToObject(pod.GetObjectMeta(), owner)
	return pod
}

func MustNewKubeClient() kubernetes.Interface {
	cfg, err := InClusterConfig()
	if err != nil {
		panic(err)
	}
	return kubernetes.NewForConfigOrDie(cfg)
}

func InClusterConfig() (*rest.Config, error) {
	// Work around https://github.com/kubernetes/kubernetes/issues/40973
	// See https://github.com/coreos/etcd-operator/issues/731#issuecomment-283804819
	if len(os.Getenv("KUBERNETES_SERVICE_HOST")) == 0 {
		addrs, err := net.LookupHost("kubernetes.default.svc")
		if err != nil {
			panic(err)
		}
		os.Setenv("KUBERNETES_SERVICE_HOST", addrs[0])
	}
	if len(os.Getenv("KUBERNETES_SERVICE_PORT")) == 0 {
		os.Setenv("KUBERNETES_SERVICE_PORT", "443")
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// Set a reasonable default request timeout
	cfg.Timeout = defaultKubeAPIRequestTimeout
	return cfg, nil
}

func IsKubernetesResourceAlreadyExistError(err error) bool {
	return apierrors.IsAlreadyExists(err)
}

func IsKubernetesResourceNotFoundError(err error) bool {
	return apierrors.IsNotFound(err)
}

// We are using internal api types for cluster related.
func ClusterListOpt(clusterName string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(LabelsForCluster(clusterName)).String(),
	}
}

func LabelsForCluster(clusterName string) map[string]string {
	return map[string]string{
		"zookeeper_cluster": clusterName,
		"app":          "zookeeper",
	}
}

func CreatePatch(o, n, datastruct interface{}) ([]byte, error) {
	oldData, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	newData, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return strategicpatch.CreateTwoWayMergePatch(oldData, newData, datastruct)
}

func PatchDeployment(kubecli kubernetes.Interface, namespace, name string, updateFunc func(*appsv1beta1.Deployment)) error {
	od, err := kubecli.AppsV1beta1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	nd := od.DeepCopy()
	updateFunc(nd)
	patchData, err := CreatePatch(od, nd, appsv1beta1.Deployment{})
	if err != nil {
		return err
	}
	_, err = kubecli.AppsV1beta1().Deployments(namespace).Patch(name, types.StrategicMergePatchType, patchData)
	return err
}

func CascadeDeleteOptions(gracePeriodSeconds int64) *metav1.DeleteOptions {
	return &metav1.DeleteOptions{
		GracePeriodSeconds: func(t int64) *int64 { return &t }(gracePeriodSeconds),
		PropagationPolicy: func() *metav1.DeletionPropagation {
			foreground := metav1.DeletePropagationForeground
			return &foreground
		}(),
	}
}

// mergeLabels merges l2 into l1. Conflicting label will be skipped.
func mergeLabels(l1, l2 map[string]string) {
	for k, v := range l2 {
		if _, ok := l1[k]; ok {
			continue
		}
		l1[k] = v
	}
}

func UniqueMemberName(clusterName string) string {
	suffix := utilrand.String(randomSuffixLength)
	if len(clusterName) > maxNameLength {
		clusterName = clusterName[:maxNameLength]
	}
	return clusterName + "-" + suffix
}
