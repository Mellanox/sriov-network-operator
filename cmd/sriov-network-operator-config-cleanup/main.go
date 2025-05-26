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

package main

import (
	"flag"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	snolog "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/log"
)

const (
	componentName = "sriov-network-operator-config-cleanup"
)

var (
	rootCmd = &cobra.Command{
		Use:   componentName,
		Short: "Removes 'default' SriovOperatorConfig",
		Long: `Removes 'default' SriovOperatorConfig in order to cleanup non-namespaced objects e.g clusterroles/clusterrolebinding/validating/mutating webhooks
		
Example: sriov-network-operator-config-cleanup -n <sriov-operator ns>`,
		RunE: runCleanupCmd,
	}
)

func main() {
	klog.InitFlags(nil)
	snolog.BindFlags(flag.CommandLine)
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	if err := rootCmd.Execute(); err != nil {
		log.Log.Error(err, "Error executing sriov-network-operator-config-cleanup")
		os.Exit(1)
	}
}
