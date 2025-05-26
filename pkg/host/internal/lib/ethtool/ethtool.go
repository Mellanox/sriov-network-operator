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

package ethtool

import (
	"github.com/safchain/ethtool"
)

func New() EthtoolLib {
	return &libWrapper{}
}

//go:generate ../../../../../bin/mockgen -destination mock/mock_ethtool.go -source ethtool.go
type EthtoolLib interface {
	// Features retrieves features of the given interface name.
	Features(ifaceName string) (map[string]bool, error)
	// FeatureNames shows supported features by their name.
	FeatureNames(ifaceName string) (map[string]uint, error)
	// Change requests a change in the given device's features.
	Change(ifaceName string, config map[string]bool) error
}

type libWrapper struct{}

// Features retrieves features of the given interface name.
func (w *libWrapper) Features(ifaceName string) (map[string]bool, error) {
	e, err := ethtool.NewEthtool()
	if err != nil {
		return nil, err
	}
	defer e.Close()
	return e.Features(ifaceName)
}

// FeatureNames shows supported features by their name.
func (w *libWrapper) FeatureNames(ifaceName string) (map[string]uint, error) {
	e, err := ethtool.NewEthtool()
	if err != nil {
		return nil, err
	}
	defer e.Close()
	return e.FeatureNames(ifaceName)
}

// Change requests a change in the given device's features.
func (w *libWrapper) Change(ifaceName string, config map[string]bool) error {
	e, err := ethtool.NewEthtool()
	if err != nil {
		return err
	}
	defer e.Close()
	return e.Change(ifaceName, config)
}
