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

package sriovnet

import (
	"github.com/k8snetworkplumbingwg/sriovnet"
)

func New() SriovnetLib {
	return &libWrapper{}
}

//go:generate ../../../../../bin/mockgen -destination mock/mock_sriovnet.go -source sriovnet.go
type SriovnetLib interface {
	// GetVfRepresentor returns representor name for VF device
	GetVfRepresentor(uplink string, vfIndex int) (string, error)
}

type libWrapper struct{}

// GetVfRepresentor returns representor name for VF device
func (w *libWrapper) GetVfRepresentor(pfName string, vfIndex int) (string, error) {
	return sriovnet.GetVfRepresentor(pfName, vfIndex)
}
