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

package featuregate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FeatureGate", func() {
	Context("IsEnabled", func() {
		It("return false for unknown feature", func() {
			Expect(New().IsEnabled("something")).To(BeFalse())
		})
	})
	Context("Init", func() {
		It("should update the state", func() {
			f := New()
			f.Init(map[string]bool{"feat1": true, "feat2": false})
			Expect(f.IsEnabled("feat1")).To(BeTrue())
			Expect(f.IsEnabled("feat2")).To(BeFalse())
		})
	})
	Context("String", func() {
		It("no features", func() {
			Expect(New().String()).To(Equal(""))
		})
		It("print feature state", func() {
			f := New()
			f.Init(map[string]bool{"feat1": true, "feat2": false})
			Expect(f.String()).To(And(ContainSubstring("feat1:true"), ContainSubstring("feat2:false")))
		})
	})
})
