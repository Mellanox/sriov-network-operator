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

package mellanox

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/log"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/host/types"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/utils"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
)

const (
	ddiBinaryName = "doca_mgmt_data_direct"
	// Source paths inside the container image.
	ddiBinarySrc = "/usr/bin/" + ddiBinaryName
	// Destination paths used after the daemon chroots to /host.
	// stageDDIAssets copies from the container filesystem to these host paths
	// (prefixed with /host) at plugin init, before any chroot takes place.
	ddiStagingBase      = "/var/lib/sriov/ddi"
	ddiStagedBinaryPath = ddiStagingBase + "/" + ddiBinaryName
	ddiStagedLibPath    = ddiStagingBase + "/lib"
)

// MellanoxVFHook implements types.VFConfigHook for Mellanox/NVIDIA NICs.
// It enables DDI (Data Direct Interface) on VFs that belong to an interface
// configured with flow_steering_mode=hmfs (Spectrum-X).
type MellanoxVFHook struct {
	kernelHelper types.KernelInterface
	utilsHelper  utils.CmdInterface
}

// OnVFUnbound is called after a VF has been unbound from its driver.
// If the PF interface is configured with flow_steering_mode=hmfs, DDI is
// enabled on the VF.
func (h *MellanoxVFHook) OnVFUnbound(
	iface *sriovnetworkv1.Interface,
	vfPciAddr string,
	_ *sriovnetworkv1.VfGroup,
) error {
	if !hasDDIRequired(iface) {
		return nil
	}

	pfPciAddr := iface.PciAddress
	fwctlDir := filepath.Join(consts.SysBusPciDevices, pfPciAddr, "fwctl")

	if _, err := os.Stat(fwctlDir); os.IsNotExist(err) {
		// Attempt to load the fwctl kernel modules.  Treat any load failure as
		// a soft-skip: a missing module means this kernel was built without
		// CONFIG_FWCTL (or lacks the mlx5 fwctl driver), so DDI is not
		// available — same outcome as the sysfs entry being absent after a
		// successful load.  Hard-failing here would abort VF configuration for
		// the entire PF on any standard kernel that does not carry fwctl.
		if err := h.kernelHelper.LoadKernelModule("fwctl"); err != nil {
			log.Log.Info("MellanoxVFHook: fwctl module not available, skipping DDI",
				"pf", pfPciAddr, "reason", err)
			return nil
		}
		if err := h.kernelHelper.LoadKernelModule("mlx5_fwctl"); err != nil {
			log.Log.Info("MellanoxVFHook: mlx5_fwctl module not available, skipping DDI",
				"pf", pfPciAddr, "reason", err)
			return nil
		}
		if _, err := os.Stat(fwctlDir); os.IsNotExist(err) {
			log.Log.Info("MellanoxVFHook: fwctl sysfs entry absent after module load, NIC does not support DDI",
				"pf", pfPciAddr)
			return nil
		}
	}

	return h.execDDI(vfPciAddr)
}

// execDDI runs doca_mgmt_data_direct to enable DDI on the given VF.
//
// In DaemonSet mode the binary and its DOCA-specific shared libraries are
// staged to the host filesystem by stageDDIAssets (called at plugin init,
// before any chroot).  execDDI runs the staged binary directly — no chroot
// manipulation is needed and vars.InChroot is never touched.
//
// In systemd mode the binary is expected on $PATH.
func (h *MellanoxVFHook) execDDI(vfPciAddr string) error {
	if vars.InChroot {
		// DaemonSet mode: binary and DOCA libs live at known host paths.
		stdout, stderr, err := h.utilsHelper.RunCommandWithEnv(
			[]string{"LD_LIBRARY_PATH=" + ddiStagedLibPath},
			ddiStagedBinaryPath,
			"set", "--vf-pci-addr", vfPciAddr, "--enabled", "true")
		return ddiResult(stdout, stderr, err, vfPciAddr)
	}
	// Systemd mode: binary is on $PATH.
	stdout, stderr, err := h.utilsHelper.RunCommand(ddiBinaryName,
		"set", "--vf-pci-addr", vfPciAddr, "--enabled", "true")
	return ddiResult(stdout, stderr, err, vfPciAddr)
}

// ddiResult interprets the output from doca_mgmt_data_direct and converts
// known non-fatal conditions (binary absent, device unsupported) into nil.
func ddiResult(stdout, stderr string, err error, vfPciAddr string) error {
	if err == nil {
		log.Log.Info("MellanoxVFHook: DDI enabled", "vf", vfPciAddr)
		return nil
	}
	// Handle both $PATH lookup failure (exec.ErrNotFound) and absolute-path
	// not-found (os.ErrNotExist) — the latter occurs when staging was skipped or
	// the staged binary was removed.
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		log.Log.Info("MellanoxVFHook: doca_mgmt_data_direct not found, skipping DDI",
			"vf", vfPciAddr)
		return nil
	}
	combined := stdout + stderr
	if strings.Contains(combined, "not supported") ||
		strings.Contains(combined, "DOCA_ERROR_NOT_SUPPORTED") ||
		strings.Contains(combined, "VHCA ID as function ID is not supported") ||
		// PF1 on CX8 in esw_multiport (Spectrum-X) does not expose a DOCA
		// fwctl device for DDI; the binary exits 1 with these messages.
		strings.Contains(combined, "Matching device not found") ||
		strings.Contains(combined, "Requested Resource Not Found") {
		log.Log.Info("MellanoxVFHook: DDI not supported on device, skipping",
			"vf", vfPciAddr)
		return nil
	}
	// The firmware rejected the DDI capability command (e.g. fwctl sysfs entry
	// exists but the NIC firmware does not implement DDI at this firmware version
	// or configuration).  Treat as a soft-skip: the VF can still be configured
	// without DDI rather than aborting the entire PF configuration.
	if strings.Contains(combined, "Failed to set data direct") ||
		strings.Contains(combined, "Failed to set HCA capabilities") {
		log.Log.Info("MellanoxVFHook: firmware rejected DDI capability, skipping",
			"vf", vfPciAddr, "output", combined)
		return nil
	}
	// A dynamic-linker error means the staged libraries are absent or
	// incomplete.  Treat this as a soft-skip so that a staging failure on one
	// boot does not abort VF configuration for the whole PF.
	if strings.Contains(combined, "error while loading shared libraries") ||
		strings.Contains(combined, "cannot open shared object file") {
		log.Log.Info("MellanoxVFHook: doca_mgmt_data_direct missing shared libraries, skipping DDI",
			"vf", vfPciAddr, "output", combined)
		return nil
	}
	// Belt-and-suspenders: some failure modes exit non-zero with no output
	// at all (e.g. very early binary errors).  Treat silent failure as a
	// soft-skip rather than a hard error to avoid infinite reconcile loops.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && combined == "" {
		exitCode := -1
		if exitErr.ProcessState != nil {
			exitCode = exitErr.ExitCode()
		}
		log.Log.Info("MellanoxVFHook: doca_mgmt_data_direct exited non-zero with no output, skipping DDI",
			"vf", vfPciAddr, "exitCode", exitCode)
		return nil
	}
	return fmt.Errorf("MellanoxVFHook: doca_mgmt_data_direct set failed for %s: %w\n%s",
		vfPciAddr, err, combined)
}

// hasDDIRequired returns true when the interface is configured for Spectrum-X
// HMFS, which requires DDI to be enabled on its VFs.
func hasDDIRequired(iface *sriovnetworkv1.Interface) bool {
	for _, p := range iface.DevlinkParams.Params {
		if p.Name == consts.DevlinkParamFlowSteeringMode &&
			p.Value == consts.FlowSteeringModeHmfs {
			return true
		}
	}
	return false
}

// systemLibBasenames are well-known glibc / GCC runtime libraries that are
// always present on the host OS and must not be overridden by container copies.
var systemLibBasenames = map[string]bool{
	"libc.so.6": true, "libm.so.6": true, "libdl.so.2": true,
	"librt.so.1": true, "libpthread.so.0": true, "libutil.so.1": true,
}

// ddiStageOnce ensures stageDDIAssets is executed at most once even when
// NewMellanoxPlugin is called multiple times (once per detected NIC).
var (
	ddiStageOnce sync.Once
	errDDIStage  error
)

// EnsureDDIStaged calls stageDDIAssets exactly once and caches the result.
func EnsureDDIStaged() error {
	ddiStageOnce.Do(func() {
		errDDIStage = stageDDIAssets()
	})
	return errDDIStage
}

// NewMellanoxVFHook creates a MellanoxVFHook ready to be registered with SetVFConfigHook.
func NewMellanoxVFHook(kernelHelper types.KernelInterface, utilsHelper utils.CmdInterface) *MellanoxVFHook {
	return &MellanoxVFHook{
		kernelHelper: kernelHelper,
		utilsHelper:  utilsHelper,
	}
}

// stageDDIAssets copies doca_mgmt_data_direct and every non-system shared
// library it transitively depends on from the container filesystem into the
// host filesystem under /host/var/lib/sriov/ddi/.
// This must be called before any chroot.
//
// ldd is run against the binary while still in the container context to
// obtain the fully-resolved path of every dependency.  copyFileMode opens
// each path with os.Open (which follows symlinks), so SONAME symlinks are
// materialized as independent real files in the staging directory.  This
// guarantees the dynamic linker can satisfy every SONAME at runtime without
// any LD_LIBRARY_PATH guessing.
func stageDDIAssets() error {
	if _, err := os.Stat(ddiBinarySrc); err != nil {
		return fmt.Errorf("stageDDIAssets: %s not found in container image: %w", ddiBinaryName, err)
	}

	dstDir := filepath.Join(consts.Host, ddiStagingBase)
	dstLib := filepath.Join(consts.Host, ddiStagedLibPath)
	if err := os.MkdirAll(dstLib, 0755); err != nil {
		return fmt.Errorf("stageDDIAssets: cannot create lib staging dir: %w", err)
	}

	if err := copyFileMode(ddiBinarySrc, filepath.Join(dstDir, ddiBinaryName), 0755); err != nil {
		return fmt.Errorf("stageDDIAssets: cannot copy binary: %w", err)
	}

	if err := stageDDILibs(dstLib); err != nil {
		return fmt.Errorf("stageDDIAssets: library staging failed; DDI binary will not run: %w", err)
	}
	return nil
}

// stageDDILibs uses ldd to enumerate every shared library that
// doca_mgmt_data_direct transitively requires and copies the non-system ones
// into dstLib.
//
// ldd is invoked while the process is still in the container root, so paths
// resolve against container-image libraries — the exact versions the binary
// was linked against.  Only the fundamental glibc libraries that are always
// available on the host are excluded (see systemLibBasenames).
func stageDDILibs(dstLib string) error {
	// ldd output lines look like one of:
	//   libfoo.so.2 => /usr/lib/x86_64-linux-gnu/libfoo.so.2 (0x7f...)
	//   linux-vdso.so.1 (0x7ffd...)           ← virtual; no file
	//   /lib64/ld-linux-x86-64.so.2 (0x7f...) ← ELF interpreter; skip
	out, err := exec.Command("ldd", ddiBinarySrc).Output()
	if err != nil {
		return fmt.Errorf("ldd %s: %w", ddiBinarySrc, err)
	}

	staged := 0
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Skip vdso, ELF interpreter, and empty lines.
		if line == "" || strings.Contains(line, "linux-vdso") ||
			strings.HasPrefix(line, "/lib") || strings.HasPrefix(line, "/lib64") {
			continue
		}

		idx := strings.Index(line, "=>")
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(line[idx+2:])
		// Remove trailing " (0x...)" load address.
		if sp := strings.Index(rest, " "); sp >= 0 {
			rest = rest[:sp]
		}
		if rest == "" || rest == "not found" {
			continue
		}

		name := filepath.Base(rest)
		if systemLibBasenames[name] {
			continue
		}

		dst := filepath.Join(dstLib, name)
		if err := copyFileMode(rest, dst, 0644); err != nil {
			log.Log.Error(err, "stageDDIAssets: failed to copy library", "lib", rest)
			continue
		}
		staged++
	}
	log.Log.Info("stageDDIAssets: staged DDI libraries", "count", staged, "dst", dstLib)
	return nil
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
