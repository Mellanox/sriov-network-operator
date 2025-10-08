// Copyright 2025 NVIDIA CORPORATION & AFFILIATES
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

package webhook

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/controllers"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
)

func TestValidate_NetworkNamespace(t *testing.T) {
	defer func(previous string) { vars.Namespace = previous }(vars.Namespace)
	vars.Namespace = "operator-namespace"

	testCases := []struct {
		name       string
		network    controllers.NetworkCRInstance
		shouldFail bool
	}{
		{
			name:       "SriovNetwork in operator namespace with empty NetworkNamespace",
			network:    &SriovNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "operator-namespace"}, Spec: SriovNetworkSpec{NetworkNamespace: ""}},
			shouldFail: false,
		},
		{
			name:       "SriovNetwork in operator namespace with custom NetworkNamespace",
			network:    &SriovNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "operator-namespace"}, Spec: SriovNetworkSpec{NetworkNamespace: "xxx"}},
			shouldFail: false,
		},
		{
			name:       "SriovNetwork in custom namespace with empty NetworkNamespace",
			network:    &SriovNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "xxx"}, Spec: SriovNetworkSpec{NetworkNamespace: ""}},
			shouldFail: false,
		},
		{
			name:       "SriovIBNetwork in operator namespace with empty NetworkNamespace",
			network:    &SriovIBNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "operator-namespace"}, Spec: SriovIBNetworkSpec{NetworkNamespace: ""}},
			shouldFail: false,
		},
		{
			name:       "SriovIBNetwork in operator namespace with custom NetworkNamespace",
			network:    &SriovIBNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "operator-namespace"}, Spec: SriovIBNetworkSpec{NetworkNamespace: "xxx"}},
			shouldFail: false,
		},
		{
			name:       "SriovIBNetwork in custom namespace with empty NetworkNamespace",
			network:    &SriovIBNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "xxx"}, Spec: SriovIBNetworkSpec{NetworkNamespace: ""}},
			shouldFail: false,
		},
		{
			name:       "OVSNetwork in operator namespace with empty NetworkNamespace",
			network:    &OVSNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "operator-namespace"}, Spec: OVSNetworkSpec{NetworkNamespace: ""}},
			shouldFail: false,
		},
		{
			name:       "OVSNetwork in operator namespace with custom NetworkNamespace",
			network:    &OVSNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "operator-namespace"}, Spec: OVSNetworkSpec{NetworkNamespace: "xxx"}},
			shouldFail: false,
		},
		{
			name:       "OVSNetwork in custom namespace with empty NetworkNamespace",
			network:    &OVSNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "xxx"}, Spec: OVSNetworkSpec{NetworkNamespace: ""}},
			shouldFail: false,
		},
		{
			name:       "SriovNetwork in custom namespace with custom NetworkNamespace",
			network:    &SriovNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "xxx"}, Spec: SriovNetworkSpec{NetworkNamespace: "yyy"}},
			shouldFail: true,
		},
		{
			name:       "SriovIBNetwork in custom namespace with custom NetworkNamespace",
			network:    &SriovIBNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "xxx"}, Spec: SriovIBNetworkSpec{NetworkNamespace: "yyy"}},
			shouldFail: true,
		},
		{
			name:       "OVSNetwork in custom namespace with custom NetworkNamespace",
			network:    &OVSNetwork{ObjectMeta: metav1.ObjectMeta{Namespace: "xxx"}, Spec: OVSNetworkSpec{NetworkNamespace: "yyy"}},
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNetworkNamespace(tc.network)
			if tc.shouldFail && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.shouldFail && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
