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

	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
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
		It("should apply default feature state", func() {
			f := NewWithDefaultFeatures(map[string]bool{"default1": true, "default2": false})
			f.Init(nil)
			Expect(f.IsEnabled("default1")).To(BeTrue())
			Expect(f.IsEnabled("default2")).To(BeFalse())
		})
		It("should override default feature state", func() {
			f := NewWithDefaultFeatures(map[string]bool{"feat1": false, "feat2": true})
			f.Init(map[string]bool{"feat1": true})
			Expect(f.IsEnabled("feat1")).To(BeTrue())
			Expect(f.IsEnabled("feat2")).To(BeTrue())
		})
		It("should apply real default feature states", func() {
			f := New()
			f.Init(nil)
			Expect(f.IsEnabled(consts.ParallelNicConfigFeatureGate)).To(BeFalse())
			Expect(f.IsEnabled(consts.ResourceInjectorMatchConditionFeatureGate)).To(BeFalse())
			Expect(f.IsEnabled(consts.MetricsExporterFeatureGate)).To(BeFalse())
			Expect(f.IsEnabled(consts.ManageSoftwareBridgesFeatureGate)).To(BeFalse())
			Expect(f.IsEnabled(consts.BlockDevicePluginUntilConfiguredFeatureGate)).To(BeTrue())
			Expect(f.IsEnabled(consts.MellanoxFirmwareResetFeatureGate)).To(BeFalse())
		})
		It("should override real default feature state", func() {
			f := New()
			f.Init(map[string]bool{consts.BlockDevicePluginUntilConfiguredFeatureGate: false})
			Expect(f.IsEnabled(consts.BlockDevicePluginUntilConfiguredFeatureGate)).To(BeFalse())
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
