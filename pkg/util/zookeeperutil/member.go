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

package zookeeperutil

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Member struct {
	// Name is of the format "clusterName-ID"
	Name string
	// Kubernetes namespace this member runs in.
	Namespace string
}

func (m *Member) Addr() string {
	return fmt.Sprintf("%s.%s.%s.svc", m.Name, clusterNameFromMemberName(m.Name), m.Namespace)
}

func (m *Member) ID() int {
	sSplit := strings.Split(m.Name, "-")
	ID, _ := strconv.Atoi(sSplit[len(sSplit)-1])
	return ID
}

type MemberSet map[string]*Member

func NewMemberSet(ms ...*Member) MemberSet {
	res := MemberSet{}
	for _, m := range ms {
		res[m.Name] = m
	}
	return res
}

// the set of all members of s1 that are not members of s2
func (ms MemberSet) Diff(other MemberSet) MemberSet {
	diff := MemberSet{}
	for n, m := range ms {
		if _, ok := other[n]; !ok {
			diff[n] = m
		}
	}
	return diff
}

// IsEqual tells whether two member sets are equal by checking
// - they have the same set of members and member equality are judged by Name only.
func (ms MemberSet) IsEqual(other MemberSet) bool {
	if ms.Size() != other.Size() {
		return false
	}
	for n := range ms {
		if _, ok := other[n]; !ok {
			return false
		}
	}
	return true
}

func (ms MemberSet) Size() int {
	return len(ms)
}

func (ms MemberSet) String() string {
	var mstring []string

	for m := range ms {
		mstring = append(mstring, m)
	}
	return strings.Join(mstring, ",")
}

func (ms MemberSet) PickOne() *Member {
	for _, m := range ms {
		return m
	}
	panic("empty")
}

func (ms MemberSet) Add(m *Member) {
	ms[m.Name] = m
}

func (ms MemberSet) Remove(name string) {
	delete(ms, name)
}

func (ms MemberSet) MaxMemberID() int {
	maxID := 0
	for _, m := range ms {
		memberID := m.ID()
		if memberID > maxID {
			maxID = memberID
		}
	}
	return maxID
}

func (ms MemberSet) ClientHostList() []string {
	hosts := make([]string, 0)
	for _, m := range ms {
		hosts = append(hosts, fmt.Sprintf("%s:2181", m.Addr()))
	}
	return hosts
}

func (ms MemberSet) ClusterConfig() []string {
	clusterConfig := make([]string, 0)
	for _, m := range ms {
		clusterConfig = append(clusterConfig, fmt.Sprintf("server.%d=%s:2888:3888:participant;%s:2181", m.ID(), m.Addr(), m.Addr()))
	}
	sort.Strings(clusterConfig)
	return clusterConfig
}

func clusterNameFromMemberName(mn string) string {
	i := strings.LastIndex(mn, "-")
	if i == -1 {
		panic(fmt.Sprintf("unexpected member name: %s", mn))
	}
	return mn[:i]
}
