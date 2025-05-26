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

package network

import (
	"testing"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/stretchr/testify/assert"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetSriovNicIPs(t *testing.T) {
	networkStatus := `[{
		"name": "network1",
		    "interface": "eth0",
		"ips": [
			"10.132.2.200"
		],
		"mac": "0a:58:0a:84:02:c8",
		"default": true,
		"dns": {}
	},{
		"name": "sriov-conformance-testing/test-multi-networkpolicy-sriov-network",
		"interface": "net1",
		"ips": [
			"2.2.2.49"
		],
		"mac": "96:a2:09:fb:4d:c3",
		"dns": {},
		"device-info": {
			"type": "pci",
			"version": "1.0.0",
			"pci": {
				"pci-address": "0000:19:00.4"
			}
		}
	}]`

	p := &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				netattdefv1.NetworkStatusAnnot: networkStatus,
			},
		},
	}

	ips, err := GetSriovNicIPs(p, "eth0")
	assert.NoError(t, err)
	assert.Contains(t, ips, "10.132.2.200")

	ips, err = GetSriovNicIPs(p, "net1")
	assert.NoError(t, err)
	assert.Contains(t, ips, "2.2.2.49")

	_, err = GetSriovNicIPs(p, "eth999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interface [eth999] not found")
}

func TestGetSriovNicIPsErrors(t *testing.T) {
	p := &k8sv1.Pod{}
	_, err := GetSriovNicIPs(p, "eth0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no annotation `k8s.v1.cni.cncf.io/network-status`")

	p = &k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/network-status": "xxx",
			},
		},
	}
	_, err = GetSriovNicIPs(p, "eth0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}
