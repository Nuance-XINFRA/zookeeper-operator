// Copyright 2017 The zookeeper-operator Authors
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

package cluster

import (
	"testing"

	api "github.com/nuance-mobility/zookeeper-operator/pkg/apis/zookeeper/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// When ZookeeperCluster update event happens, local object ref should be updated.
func TestUpdateEventUpdateLocalClusterObj(t *testing.T) {
	oldVersion := "123"
	newVersion := "321"

	oldObj := &api.ZookeeperCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: oldVersion,
			Name:            "test",
			Namespace:       metav1.NamespaceDefault,
		},
	}
	newObj := oldObj.DeepCopy()
	newObj.ResourceVersion = newVersion

	c := &Cluster{
		cluster: oldObj,
	}
	e := &clusterEvent{
		typ:     eventModifyCluster,
		cluster: newObj,
	}

	err := c.handleUpdateEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	if c.cluster.ResourceVersion != newVersion {
		t.Errorf("expect version=%s, get=%s", newVersion, c.cluster.ResourceVersion)
	}
}
