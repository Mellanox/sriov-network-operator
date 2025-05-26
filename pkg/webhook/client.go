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

package webhook

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
)

var client runtimeclient.Client
var kubeclient *kubernetes.Clientset

func SetupInClusterClient() error {
	var err error
	var config *rest.Config

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		log.Log.Error(nil, "fail to create config")
		return err
	}

	client, err = runtimeclient.New(config, runtimeclient.Options{Scheme: vars.Scheme})
	if err != nil {
		log.Log.Error(nil, "fail to setup client")
		return err
	}

	kubeclient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Log.Error(nil, "fail to setup kubernetes client")
		return err
	}

	return nil
}
