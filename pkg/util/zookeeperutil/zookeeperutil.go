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
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	/* TODO: @MDF: The ZK client has multiple issues which need addressing:
	 * - it identifies as an old client version which causes WARNs in ZK itself
	 * - if a server in the list of hosts is inaccessible it segfaults
	 */
	"github.com/blafrisch/go-zookeeper/zk"
)

func GetClusterConfig(hosts []string) ([]string, error) {
	conn, _, err := zk.Connect(hosts, time.Second)
	defer conn.Close()
	if err != nil {
		glog.Error("Failed to connect to ZK hosts: ", hosts)
		return nil, err
	}

	data, _, err := conn.Get("/zookeeper/config")
	if err != nil {
		return nil, err
	}

	// data is a []byte, we must convert it to a string
	dataStr := string(data)
	// the config data has servers first, last line is the version
	configDataArr := strings.Split(dataStr, "\n")
	clusterConfig := configDataArr[:len(configDataArr)-1]
	sort.Strings(clusterConfig)

	return clusterConfig, nil
}

func ReconfigureCluster(hosts []string, desiredConfig []string) ([]string, error) {
	conn, _, err := zk.Connect(hosts, time.Second)
	defer conn.Close()
	if err != nil {
		glog.Error("Failed to connect to ZK hosts: ", hosts)
		return nil, err
	}

	// args are (joiningServers string, leavingServers string, newMembers string, fromConfig int64)
	// only required params are the first two if doing an incremental change
	//   or the third param if doing a non-incremental
	newMembers := strings.Join(desiredConfig, ",")
	data, _, err := conn.Reconfig("", "", newMembers, -1)
	if err != nil {
		glog.Error("Failed to push reconfig: ", newMembers)
		return nil, err
	}

	// data is a []byte, we must convert it to a string
	dataStr := string(data)
	// the config data has servers first, last line is the version
	configDataArr := strings.Split(dataStr, "\n")
	clusterConfig := configDataArr[:len(configDataArr)-1]
	sort.Strings(clusterConfig)

	return clusterConfig, nil
}