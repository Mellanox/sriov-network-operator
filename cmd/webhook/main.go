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
	componentName = "sriov-network-operator-webhook"
)

var (
	rootCmd = &cobra.Command{
		Use:   componentName,
		Short: "Run SR-IoV Operator Webhook Daemon",
		Long:  "Run Webhook Daemon which validates/mutates the Custom Resource of the SR-IoV Network Operator",
	}
)

func init() {
	klog.InitFlags(nil)
	snolog.BindFlags(flag.CommandLine)
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Log.Error(err, "Error executing sriov-network-operator-webhook")
		os.Exit(1)
	}
}
