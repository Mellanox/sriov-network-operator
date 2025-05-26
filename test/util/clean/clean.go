// Copyright 2025 sriov-network-device-plugin authors
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
//
// SPDX-License-Identifier: Apache-2.0

package clean

import (
	"fmt"
	"os"
	"time"

	"github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/client"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/cluster"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/namespaces"
)

var RestoreNodeDrainState bool

// All cleans all the dangling resources created by conformance tests.
// This includes pods, networks, policies and namespaces.
func All() error {
	operatorNamespace, found := os.LookupEnv("OPERATOR_NAMESPACE")
	if !found {
		operatorNamespace = "openshift-sriov-network-operator"
	}
	clients := client.New("")
	if clients == nil {
		return fmt.Errorf("failed to create ClientSet")
	}

	if RestoreNodeDrainState {
		err := cluster.SetDisableNodeDrainState(clients, operatorNamespace, false)
		if err != nil {
			return fmt.Errorf("failed to restore node drain state %v", err)
		}
	}

	err := namespaces.DeleteAndWait(clients, namespaces.Test, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to delete sriov tests namespace %v", err)
	}

	err = namespaces.Clean(operatorNamespace, namespaces.Test, clients, false)
	if err != nil {
		return fmt.Errorf("failed to clean sriov resources %v", err)
	}

	return nil
}
