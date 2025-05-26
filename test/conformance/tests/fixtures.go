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

package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/clean"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/cluster"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/discovery"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/namespaces"
)

var sriovInfos *cluster.EnabledNodes

var _ = BeforeSuite(func() {
	err := clean.All()
	Expect(err).NotTo(HaveOccurred())

	isSingleNode, err := cluster.IsSingleNode(clients)
	Expect(err).ToNot(HaveOccurred())
	if isSingleNode {
		disableDrainState, err := cluster.GetNodeDrainState(clients, operatorNamespace)
		Expect(err).ToNot(HaveOccurred())
		if discovery.Enabled() {
			if !disableDrainState {
				Fail("SriovOperatorConfig DisableDrain property must be enabled in a single node environment")
			}
		}
		snoTimeoutMultiplier = 1
		if !disableDrainState {
			err = cluster.SetDisableNodeDrainState(clients, operatorNamespace, true)
			Expect(err).ToNot(HaveOccurred())
			clean.RestoreNodeDrainState = true
		}
	}

	if !isFeatureFlagEnabled(consts.ResourceInjectorMatchConditionFeatureGate) {
		setFeatureFlag(consts.ResourceInjectorMatchConditionFeatureGate, true)
	}

	err = namespaces.Create(namespaces.Test, clients)
	Expect(err).ToNot(HaveOccurred())
	err = namespaces.Clean(operatorNamespace, namespaces.Test, clients, discovery.Enabled())
	Expect(err).ToNot(HaveOccurred())
	WaitForSRIOVStable()
	sriovInfos, err = cluster.DiscoverSriov(clients, operatorNamespace)
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	err := clean.All()
	Expect(err).NotTo(HaveOccurred())
})
