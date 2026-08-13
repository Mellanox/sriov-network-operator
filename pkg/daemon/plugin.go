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
	plugin "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
)

// mutuallyExclusivePlugins lists groups of plugin names where only one should
// be active at a time. The first name in each group takes precedence when
// multiple are loaded and none is explicitly disabled.
var mutuallyExclusivePlugins = [][]string{
	{"NvidiaPlugin", "mellanox"},
}

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

	// Within each mutually-exclusive group, keep only the first active plugin.
	// This lets disabledPlugins select between alternatives (e.g. NvidiaPlugin
	// vs mellanox for 15b3 devices) while defaulting to the preferred one.
	dn.additionalPlugins = enforceMutualExclusion(dn.additionalPlugins, mutuallyExclusivePlugins)

	additionalPluginsName := make([]string, len(dn.additionalPlugins))
	for idx, plugin := range dn.additionalPlugins {
		additionalPluginsName[idx] = plugin.Name()
	}

	log.Log.Info("loaded plugins", "mainPlugin", dn.mainPlugin.Name(), "additionalPlugins", additionalPluginsName)
	return nil
}

// enforceMutualExclusion removes lower-priority plugins from groups where
// multiple members are active. Within each group, the first-listed name wins.
func enforceMutualExclusion(plugins []plugin.VendorPlugin, groups [][]string) []plugin.VendorPlugin {
	activeNames := map[string]bool{}
	for _, p := range plugins {
		activeNames[p.Name()] = true
	}

	suppressed := map[string]bool{}
	for _, group := range groups {
		for i, name := range group {
			if activeNames[name] {
				// This is the highest-priority active member; suppress the rest.
				for _, lower := range group[i+1:] {
					if activeNames[lower] {
						log.Log.V(2).Info("mutually exclusive plugin suppressed",
							"kept", name, "suppressed", lower)
						suppressed[lower] = true
					}
				}
				break
			}
		}
	}

	if len(suppressed) == 0 {
		return plugins
	}
	result := plugins[:0]
	for _, p := range plugins {
		if !suppressed[p.Name()] {
			result = append(result, p)
		}
	}
	return result
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
