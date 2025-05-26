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

package daemon

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
)

func (dn *NodeReconciler) loadPlugins(ns *sriovnetworkv1.SriovNetworkNodeState, disabledPlugins []string) error {
	funcLog := log.Log.WithName("loadPlugins").WithValues("platform", vars.PlatformType, "orchestrator", vars.ClusterType)
	funcLog.Info("loading plugins", "disabled", disabledPlugins)

	mainPlugin, additionalPlugins, err := dn.platformInterface.GetVendorPlugins(ns)
	if err != nil {
		funcLog.Error(err, "Failed to load plugins", "platform", vars.PlatformType, "orchestrator", vars.ClusterType)
		return err
	}

	// Check if the main plugin is disabled - this is not allowed
	if isPluginDisabled(mainPlugin.Name(), disabledPlugins) {
		return fmt.Errorf("main plugin %s cannot be disabled", mainPlugin.Name())
	}
	dn.mainPlugin = mainPlugin

	for _, plugin := range additionalPlugins {
		if !isPluginDisabled(plugin.Name(), disabledPlugins) {
			dn.additionalPlugins = append(dn.additionalPlugins, plugin)
		}
	}

	additionalPluginsName := make([]string, len(dn.additionalPlugins))
	for idx, plugin := range dn.additionalPlugins {
		additionalPluginsName[idx] = plugin.Name()
	}

	log.Log.Info("loaded plugins", "mainPlugin", dn.mainPlugin.Name(), "additionalPlugins", additionalPluginsName)
	return nil
}

func isPluginDisabled(pluginName string, disabledPlugins []string) bool {
	for _, p := range disabledPlugins {
		if p == pluginName {
			log.Log.V(2).Info("plugin is disabled", "name", pluginName)
			return true
		}
	}
	return false
}
