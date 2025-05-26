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

package helper

import (
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/host"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/host/store"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/utils"
	mlx "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vendors/mellanox"
)

//go:generate ../../bin/mockgen -destination mock/mock_helper.go -source host.go
type HostHelpersInterface interface {
	utils.CmdInterface
	host.HostManagerInterface
	store.ManagerInterface
	mlx.MellanoxInterface
}

type hostHelpers struct {
	utils.CmdInterface
	host.HostManagerInterface
	store.ManagerInterface
	mlx.MellanoxInterface
}

func NewDefaultHostHelpers() (HostHelpersInterface, error) {
	utilsHelper := utils.New()
	mlxHelper := mlx.New(utilsHelper)
	hostManager, err := host.NewHostManager(utilsHelper)
	if err != nil {
		log.Log.Error(err, "failed to create host manager")
		return nil, err
	}
	storeManager, err := store.NewManager()
	if err != nil {
		log.Log.Error(err, "failed to create store manager")
		return nil, err
	}

	return &hostHelpers{
		utilsHelper,
		hostManager,
		storeManager,
		mlxHelper}, nil
}
