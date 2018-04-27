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

package cluster

import (
	"fmt"
	"strings"

	"github.com/nuance-mobility/zookeeper-operator/pkg/util/zookeeperutil"

	"k8s.io/api/core/v1"
)

func (c *Cluster) updateMembers(known zookeeperutil.MemberSet) error {
	resp, err := zookeeperutil.GetClusterConfig(known.ClientHostList())
	if err != nil {
		return err
	}

	members := zookeeperutil.MemberSet{}
	for _, serverConfig := range resp {
		leaderClientSplit := strings.Split(serverConfig, ";")
		clientHostname := strings.Split(leaderClientSplit[1], ":")[0]
		clientName := strings.Split(clientHostname, ".")[0]
		members[clientName] = &zookeeperutil.Member{
			Name:         clientName,
			Namespace:    c.cluster.Namespace,
		}

	}
	c.members = members
	return nil
}

func (c *Cluster) newMember() *zookeeperutil.Member {
	name := fmt.Sprintf("%s-%d", c.cluster.Name, c.members.MaxMemberID()+1)
	return &zookeeperutil.Member{
		Name:         name,
		Namespace:    c.cluster.Namespace,
	}
}

func podsToMemberSet(pods []*v1.Pod) zookeeperutil.MemberSet {
	members := zookeeperutil.MemberSet{}
	for _, pod := range pods {
		m := &zookeeperutil.Member{Name: pod.Name, Namespace: pod.Namespace}
		members.Add(m)
	}
	return members
}
