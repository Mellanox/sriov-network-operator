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
	"fmt"

	v1 "k8s.io/api/admission/v1"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/controllers"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
)

func validateSriovNetwork(cr *sriovnetworkv1.SriovNetwork, operation v1.Operation) (bool, []string, error) {
	err := validateNetworkNamespace(cr)
	if err != nil {
		return false, nil, err
	}
	return true, nil, nil
}

func validateSriovIBNetwork(cr *sriovnetworkv1.SriovIBNetwork, operation v1.Operation) (bool, []string, error) {
	err := validateNetworkNamespace(cr)
	if err != nil {
		return false, nil, err
	}
	return true, nil, nil
}

func validateOVSNetwork(cr *sriovnetworkv1.OVSNetwork, operation v1.Operation) (bool, []string, error) {
	err := validateNetworkNamespace(cr)
	if err != nil {
		return false, nil, err
	}
	return true, nil, nil
}

func validateNetworkNamespace(cr controllers.NetworkCRInstance) error {
	if cr.GetNamespace() != vars.Namespace && cr.NetworkNamespace() != "" {
		return fmt.Errorf(".Spec.NetworkNamespace field can't be specified if the resource is not in the %s namespace", vars.Namespace)
	}

	return nil
}
