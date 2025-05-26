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

package log

import (
	"io"
	"os"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var tempLogFile *os.File
var origWriter io.Writer

var _ = g.Describe("Logging", func() {

	g.BeforeEach(func() {
		err := os.Truncate(tempLogFile.Name(), 0)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("LogLevel 0", func() {

		log.Log.Info("test level 0")
		log.Log.V(1).Info("test level 1")
		log.Log.V(2).Info("test level 2")

		out, err := os.ReadFile(tempLogFile.Name())
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(string(out)).Should(o.ContainSubstring("test level 0"))
		o.Expect(string(out)).ShouldNot(o.ContainSubstring("test level 1"))
		o.Expect(string(out)).ShouldNot(o.ContainSubstring("test level 2"))
	})

	g.It("LogLevel 1", func() {

		SetLogLevel(1)

		log.Log.Info("test level 0")
		log.Log.V(1).Info("test level 1")
		log.Log.V(2).Info("test level 2")

		out, err := os.ReadFile(tempLogFile.Name())
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(string(out)).Should(o.ContainSubstring("test level 0"))
		o.Expect(string(out)).Should(o.ContainSubstring("test level 1"))
		o.Expect(string(out)).ShouldNot(o.ContainSubstring("test level 2"))
	})

	g.It("LogLevel 2", func() {

		SetLogLevel(2)

		log.Log.Info("test level 0")
		log.Log.V(1).Info("test level 1")
		log.Log.V(2).Info("test level 2")

		out, err := os.ReadFile(tempLogFile.Name())
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(string(out)).Should(o.ContainSubstring("test level 0"))
		o.Expect(string(out)).Should(o.ContainSubstring("test level 1"))
		o.Expect(string(out)).Should(o.ContainSubstring("test level 2"))
	})
})

func TestLogging(t *testing.T) {
	o.RegisterFailHandler(g.Fail)
	g.RunSpecs(t, "Logging Suite")
}

var _ = g.BeforeSuite(func() {
	var err error
	tempLogFile, err = os.CreateTemp("", "zap-output")
	o.Expect(err).NotTo(o.HaveOccurred())
	origWriter = Options.DestWriter
	Options.DestWriter = tempLogFile

	InitLog()
})

var _ = g.AfterSuite(func() {
	Options.DestWriter = origWriter
	o.Expect(tempLogFile.Close()).To(o.Succeed())
	o.Expect(os.RemoveAll(tempLogFile.Name())).To(o.Succeed())

	SetLogLevel(2)
})
