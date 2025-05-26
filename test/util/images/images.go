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

package images

import (
	"fmt"
	"os"
)

var (
	registry      string
	cnfTestsImage string
)

func init() {
	registry = os.Getenv("IMAGE_REGISTRY")
	if registry == "" {
		registry = "quay.io/openshift-kni/"
	}

	cnfTestsImage = os.Getenv("CNF_TESTS_IMAGE")
	if cnfTestsImage == "" {
		cnfTestsImage = "cnf-tests:4.7"
	}
}

// Test returns the test image to be used
func Test() string {
	return fmt.Sprintf("%s%s", registry, cnfTestsImage)
}
