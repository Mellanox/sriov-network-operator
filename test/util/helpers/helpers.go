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

package helpers

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/fakefilesystem"
)

// GinkgoConfigureFakeFS configure fake filesystem by setting vars.FilesystemRoot
// and register Ginkgo DeferCleanup handler to clean the fs when test completed
func GinkgoConfigureFakeFS(f *fakefilesystem.FS) {
	var (
		cleanFakeFs func()
		err         error
	)
	vars.FilesystemRoot, cleanFakeFs, err = f.Use()
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(cleanFakeFs)
}

// GinkgoAssertFileContentsEquals check that content of the file
// match the expected value.
// prepends vars.FilesystemRoot to the file path to be compatible with
// GinkgoConfigureFakeFS function
func GinkgoAssertFileContentsEquals(path, expectedContent string) {
	d, err := os.ReadFile(filepath.Join(vars.FilesystemRoot, path))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, string(d)).To(Equal(expectedContent))
}

// GinkgoAssertFileDoesNotExist check that the file exist
// prepends vars.FilesystemRoot to the file path to be compatible with
// GinkgoConfigureFakeFS function
func GinkgoAssertFileDoesNotExist(path string) {
	_, err := os.Stat(filepath.Join(vars.FilesystemRoot, path))
	ExpectWithOffset(1, err).To(HaveOccurred())
	ExpectWithOffset(1, os.IsNotExist(err)).To(BeTrue())
}
