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

package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	intstrutil "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/render"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
)

const (
	LASTNETWORKNAMESPACE        = "operator.sriovnetwork.openshift.io/last-network-namespace"
	NETATTDEFFINALIZERNAME      = "netattdef.finalizers.sriovnetwork.openshift.io"
	POOLCONFIGFINALIZERNAME     = "poolconfig.finalizers.sriovnetwork.openshift.io"
	OPERATORCONFIGFINALIZERNAME = "operatorconfig.finalizers.sriovnetwork.openshift.io"
	ESwithModeLegacy            = "legacy"
	ESwithModeSwitchDev         = "switchdev"

	SriovCniStateEnable  = "enable"
	SriovCniStateDisable = "disable"
	SriovCniStateAuto    = "auto"
	SriovCniStateOff     = "off"
	SriovCniStateOn      = "on"
	SriovCniIpam         = "\"ipam\""
	SriovCniIpamEmpty    = SriovCniIpam + ":{}"
)

const invalidVfIndex = -1

var ManifestsPath = "./bindata/manifests/cni-config"
var log = logf.Log.WithName("sriovnetwork")

// NicIDMap contains supported mapping of IDs with each in the format of:
// Vendor ID, Physical Function Device ID, Virtual Function Device ID
var NicIDMap = []string{}

var InitialState SriovNetworkNodeState

// NetFilterType Represents the NetFilter tags to be used
type NetFilterType int

const (
	// OpenstackNetworkID network UUID
	OpenstackNetworkID NetFilterType = iota

	SupportedNicIDConfigmap = "supported-nic-ids"
)

type ConfigurationModeType string

const (
	DaemonConfigurationMode  ConfigurationModeType = "daemon"
	SystemdConfigurationMode ConfigurationModeType = "systemd"
)

func (e NetFilterType) String() string {
	switch e {
	case OpenstackNetworkID:
		return "openstack/NetworkID"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

func InitNicIDMapFromConfigMap(client kubernetes.Interface, namespace string) error {
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(
		context.Background(),
		SupportedNicIDConfigmap,
		metav1.GetOptions{},
	)
	// if the configmap does not exist, return false
	if err != nil {
		return err
	}
	for _, v := range cm.Data {
		NicIDMap = append(NicIDMap, v)
	}

	return nil
}

func InitNicIDMapFromList(idList []string) {
	NicIDMap = append(NicIDMap, idList...)
}

func IsSupportedVendor(vendorID string) bool {
	for _, n := range NicIDMap {
		ids := strings.Split(n, " ")
		if vendorID == ids[0] {
			return true
		}
	}
	return false
}

func IsSupportedDevice(deviceID string) bool {
	for _, n := range NicIDMap {
		ids := strings.Split(n, " ")
		if deviceID == ids[1] {
			return true
		}
	}
	return false
}

func IsSupportedModel(vendorID, deviceID string) bool {
	for _, n := range NicIDMap {
		ids := strings.Split(n, " ")
		if vendorID == ids[0] && deviceID == ids[1] {
			return true
		}
	}
	log.Info("IsSupportedModel(): found unsupported model", "vendorId:", vendorID, "deviceId:", deviceID)
	return false
}

func IsVfSupportedModel(vendorID, deviceID string) bool {
	for _, n := range NicIDMap {
		ids := strings.Split(n, " ")
		if vendorID == ids[0] && deviceID == ids[2] {
			return true
		}
	}
	log.Info("IsVfSupportedModel(): found unsupported VF model", "vendorId:", vendorID, "deviceId:", deviceID)
	return false
}

func GetSupportedVfIds() []string {
	var vfIds []string
	for _, n := range NicIDMap {
		ids := strings.Split(n, " ")
		vfID := "0x" + ids[2]
		if !StringInArray(vfID, vfIds) {
			vfIds = append(vfIds, vfID)
		}
	}
	// return a sorted slice so that udev rule is stable
	sort.Slice(vfIds, func(i, j int) bool {
		ip, _ := strconv.ParseInt(vfIds[i], 0, 32)
		jp, _ := strconv.ParseInt(vfIds[j], 0, 32)
		return ip < jp
	})
	return vfIds
}

func GetVfDeviceID(deviceID string) string {
	for _, n := range NicIDMap {
		ids := strings.Split(n, " ")
		if deviceID == ids[1] {
			return ids[2]
		}
	}
	return ""
}

func IsSwitchdevModeSpec(spec SriovNetworkNodeStateSpec) bool {
	return ContainsSwitchdevInterface(spec.Interfaces)
}

// ContainsSwitchdevInterface returns true if provided interface list contains interface
// with switchdev configuration
func ContainsSwitchdevInterface(interfaces []Interface) bool {
	for _, iface := range interfaces {
		if iface.EswitchMode == ESwithModeSwitchDev {
			return true
		}
	}
	return false
}

// GetEswitchModeFromSpec returns ESwitchMode from the interface spec, returns legacy if not set
func GetEswitchModeFromSpec(ifaceSpec *Interface) string {
	if ifaceSpec.EswitchMode == "" {
		return ESwithModeLegacy
	}
	return ifaceSpec.EswitchMode
}

// GetEswitchModeFromStatus returns ESwitchMode from the interface status, returns legacy if not set
func GetEswitchModeFromStatus(ifaceStatus *InterfaceExt) string {
	if ifaceStatus.EswitchMode == "" {
		return ESwithModeLegacy
	}
	return ifaceStatus.EswitchMode
}

func NeedToUpdateSriov(ifaceSpec *Interface, ifaceStatus *InterfaceExt) bool {
	if ifaceSpec.Mtu > 0 {
		mtu := ifaceSpec.Mtu
		if mtu > ifaceStatus.Mtu {
			log.V(0).Info("NeedToUpdateSriov(): MTU needs update", "desired", mtu, "current", ifaceStatus.Mtu)
			return true
		}
	}
	currentEswitchMode := GetEswitchModeFromStatus(ifaceStatus)
	desiredEswitchMode := GetEswitchModeFromSpec(ifaceSpec)
	if currentEswitchMode != desiredEswitchMode {
		log.V(0).Info("NeedToUpdateSriov(): EswitchMode needs update", "desired", desiredEswitchMode, "current", currentEswitchMode)
		return true
	}
	if ifaceSpec.NumVfs != ifaceStatus.NumVfs {
		log.V(0).Info("NeedToUpdateSriov(): NumVfs needs update", "desired", ifaceSpec.NumVfs, "current", ifaceStatus.NumVfs)
		return true
	}

	if ifaceStatus.LinkAdminState == consts.LinkAdminStateDown {
		log.V(0).Info("NeedToUpdateSriov(): PF link status needs update", "desired to include", "up", "current", ifaceStatus.LinkAdminState)
		return true
	}

	if ifaceSpec.NumVfs > 0 {
		for _, vfStatus := range ifaceStatus.VFs {
			for _, groupSpec := range ifaceSpec.VfGroups {
				if IndexInRange(vfStatus.VfID, groupSpec.VfRange) {
					if vfStatus.Driver == "" {
						log.V(0).Info("NeedToUpdateSriov(): Driver needs update - has no driver",
							"desired", groupSpec.DeviceType)
						return true
					}
					if groupSpec.DeviceType != "" && groupSpec.DeviceType != consts.DeviceTypeNetDevice {
						if groupSpec.DeviceType != vfStatus.Driver {
							log.V(0).Info("NeedToUpdateSriov(): Driver needs update",
								"desired", groupSpec.DeviceType, "current", vfStatus.Driver)
							return true
						}
					} else {
						if StringInArray(vfStatus.Driver, vars.DpdkDrivers) {
							log.V(0).Info("NeedToUpdateSriov(): Driver needs update",
								"desired", groupSpec.DeviceType, "current", vfStatus.Driver)
							return true
						}
						if vfStatus.Mtu != 0 && groupSpec.Mtu != 0 && vfStatus.Mtu != groupSpec.Mtu {
							log.V(0).Info("NeedToUpdateSriov(): VF MTU needs update",
								"vf", vfStatus.VfID, "desired", groupSpec.Mtu, "current", vfStatus.Mtu)
							return true
						}

						if (strings.EqualFold(ifaceStatus.LinkType, consts.LinkTypeETH) && groupSpec.IsRdma) || strings.EqualFold(ifaceStatus.LinkType, consts.LinkTypeIB) {
							// We do this check only if a Node GUID is set to ensure that we were able to read the
							// Node GUID. We intentionally skip empty Node GUID in vfStatus because this may happen
							// when the VF is allocated to a workload.
							if vfStatus.GUID == consts.UninitializedNodeGUID {
								log.V(0).Info("NeedToUpdateSriov(): VF GUID needs update",
									"vf", vfStatus.VfID, "current", vfStatus.GUID)
								return true
							}
						}
						// this is needed to be sure the admin mac address is configured as expected
						if ifaceSpec.ExternallyManaged {
							log.V(0).Info("NeedToUpdateSriov(): need to update the device as it's externally manage",
								"device", ifaceStatus.PciAddress)
							return true
						}
					}
					if groupSpec.VdpaType != vfStatus.VdpaType {
						log.V(0).Info("NeedToUpdateSriov(): VF VdpaType mismatch",
							"desired", groupSpec.VdpaType, "current", vfStatus.VdpaType)
						return true
					}
					break
				}
			}
		}
	}
	return false
}

type ByPriority []SriovNetworkNodePolicy

func (a ByPriority) Len() int {
	return len(a)
}

func (a ByPriority) Less(i, j int) bool {
	if a[i].Spec.Priority != a[j].Spec.Priority {
		return a[i].Spec.Priority > a[j].Spec.Priority
	}
	return a[i].GetName() < a[j].GetName()
}

func (a ByPriority) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// Match check if node is selected by NodeSelector
func (p *SriovNetworkNodePolicy) Selected(node *corev1.Node) bool {
	for k, v := range p.Spec.NodeSelector {
		if nv, ok := node.Labels[k]; ok && nv == v {
			continue
		}
		return false
	}
	return true
}

func StringInArray(val string, array []string) bool {
	for i := range array {
		if array[i] == val {
			return true
		}
	}
	return false
}

func RemoveString(s string, slice []string) (result []string, found bool) {
	if len(slice) != 0 {
		for _, item := range slice {
			if item == s {
				found = true
				continue
			}
			result = append(result, item)
		}
	}
	return
}

func UniqueAppend(inSlice []string, strings ...string) []string {
	for _, s := range strings {
		if !StringInArray(s, inSlice) {
			inSlice = append(inSlice, s)
		}
	}
	return inSlice
}

// Apply policy to SriovNetworkNodeState CR
func (p *SriovNetworkNodePolicy) Apply(state *SriovNetworkNodeState, equalPriority bool) error {
	s := p.Spec.NicSelector
	if s.IsEmpty() {
		// Empty NicSelector match none
		return nil
	}
	for _, iface := range state.Status.Interfaces {
		if s.Selected(&iface) {
			log.Info("Update interface", "name:", iface.Name)
			result := Interface{
				PciAddress:        iface.PciAddress,
				Mtu:               p.Spec.Mtu,
				Name:              iface.Name,
				LinkType:          p.Spec.LinkType,
				EswitchMode:       p.Spec.EswitchMode,
				NumVfs:            p.Spec.NumVfs,
				ExternallyManaged: p.Spec.ExternallyManaged,
			}
			if p.Spec.NumVfs > 0 {
				group, err := p.generatePfNameVfGroup(&iface)
				if err != nil {
					return err
				}
				result.VfGroups = []VfGroup{*group}
				found := false
				for i := range state.Spec.Interfaces {
					if state.Spec.Interfaces[i].PciAddress == result.PciAddress {
						found = true
						state.Spec.Interfaces[i].mergeConfigs(&result, equalPriority)
						state.Spec.Interfaces[i] = result
						break
					}
				}
				if !found {
					state.Spec.Interfaces = append(state.Spec.Interfaces, result)
				}
			}
		}
	}
	return nil
}

// ApplyBridgeConfig applies bridge configuration from the policy to the provided state
func (p *SriovNetworkNodePolicy) ApplyBridgeConfig(state *SriovNetworkNodeState) error {
	if p.Spec.NicSelector.IsEmpty() {
		// Empty NicSelector match none
		return nil
	}
	// sanity check the policy
	if !p.Spec.Bridge.IsEmpty() {
		if p.Spec.EswitchMode != ESwithModeSwitchDev {
			return fmt.Errorf("eSwitchMode must be switchdev to use software bridge management")
		}
		if p.Spec.LinkType != "" && !strings.EqualFold(p.Spec.LinkType, consts.LinkTypeETH) {
			return fmt.Errorf("linkType must be eth or ETH to use software bridge management")
		}
		if p.Spec.ExternallyManaged {
			return fmt.Errorf("software bridge management can't be used when link is externally managed")
		}
	}
	for _, iface := range state.Status.Interfaces {
		if p.Spec.NicSelector.Selected(&iface) {
			if p.Spec.Bridge.OVS == nil {
				// The policy has no OVS bridge config, this means that the node's state should have no managed OVS bridges for the interfaces that match the policy.
				// Currently PF to OVS bridge mapping is always 1 to 1 (bonding is not supported at the moment), meaning we can remove the OVS bridge
				// config from the node's state if it has the interface (that matches "empty-bridge" policy) in the uplink section.
				state.Spec.Bridges.OVS = slices.DeleteFunc(state.Spec.Bridges.OVS, func(br OVSConfigExt) bool {
					return slices.ContainsFunc(br.Uplinks, func(uplink OVSUplinkConfigExt) bool {
						return uplink.PciAddress == iface.PciAddress
					})
				})
				if len(state.Spec.Bridges.OVS) == 0 {
					state.Spec.Bridges.OVS = nil
				}
				continue
			}
			ovsBridge := OVSConfigExt{
				Name:   GenerateBridgeName(&iface),
				Bridge: p.Spec.Bridge.OVS.Bridge,
				Uplinks: []OVSUplinkConfigExt{{
					PciAddress: iface.PciAddress,
					Name:       iface.Name,
					Interface:  p.Spec.Bridge.OVS.Uplink.Interface,
				}},
			}
			if p.Spec.Mtu > 0 {
				mtu := p.Spec.Mtu
				ovsBridge.Uplinks[0].Interface.MTURequest = &mtu
			}
			log.Info("Update bridge for interface", "name", iface.Name, "bridge", ovsBridge.Name)

			// We need to keep slices with bridges ordered to avoid unnecessary updates in the K8S API.
			// Use binary search to insert (or update) the bridge config to the right place in the slice to keep it sorted.
			pos, exist := slices.BinarySearchFunc(state.Spec.Bridges.OVS, ovsBridge, func(x, y OVSConfigExt) int {
				return strings.Compare(x.Name, y.Name)
			})
			if exist {
				state.Spec.Bridges.OVS[pos] = ovsBridge
			} else {
				state.Spec.Bridges.OVS = slices.Insert(state.Spec.Bridges.OVS, pos, ovsBridge)
			}
		}
	}
	return nil
}

// mergeConfigs merges configs from multiple polices where the last one has the
// highest priority. This merge is dependent on: 1. SR-IOV partition is
// configured with the #-notation in pfName, 2. The VF groups are
// non-overlapping or SR-IOV policies have the same priority.
func (iface Interface) mergeConfigs(input *Interface, equalPriority bool) {
	m := false
	// merge VF groups (input.VfGroups already contains the highest priority):
	// - skip group with same ResourceName,
	// - skip overlapping groups (use only highest priority)
	for _, gr := range iface.VfGroups {
		if gr.ResourceName == input.VfGroups[0].ResourceName || gr.isVFRangeOverlapping(input.VfGroups[0]) {
			continue
		}
		m = true
		input.VfGroups = append(input.VfGroups, gr)
	}

	if !equalPriority && !m {
		return
	}

	// mtu configuration we take the highest value
	if input.Mtu < iface.Mtu {
		input.Mtu = iface.Mtu
	}
	if input.NumVfs < iface.NumVfs {
		input.NumVfs = iface.NumVfs
	}
}

func (gr VfGroup) isVFRangeOverlapping(group VfGroup) bool {
	rngSt, rngEnd, err := parseRange(gr.VfRange)
	if err != nil {
		return false
	}
	rngSt2, rngEnd2, err := parseRange(group.VfRange)
	if err != nil {
		return false
	}
	// compare minimal range has overlap
	if rngSt < rngSt2 {
		return IndexInRange(rngSt2, gr.VfRange) || IndexInRange(rngEnd2, gr.VfRange)
	}
	return IndexInRange(rngSt, group.VfRange) || IndexInRange(rngEnd, group.VfRange)
}

func (p *SriovNetworkNodePolicy) generatePfNameVfGroup(iface *InterfaceExt) (*VfGroup, error) {
	var err error
	pfName := ""
	var rngStart, rngEnd int
	found := false
	for _, selector := range p.Spec.NicSelector.PfNames {
		pfName, rngStart, rngEnd, err = ParseVfRange(selector)
		if err != nil {
			log.Error(err, "Unable to parse PF Name.")
			return nil, err
		}
		if pfName == iface.Name {
			found = true
			if rngStart == invalidVfIndex && rngEnd == invalidVfIndex {
				rngStart, rngEnd = 0, p.Spec.NumVfs-1
			}
			break
		}
	}
	if !found {
		// assign the default vf index range if the pfName is not specified by the nicSelector
		rngStart, rngEnd = 0, p.Spec.NumVfs-1
	}
	rng := strconv.Itoa(rngStart) + "-" + strconv.Itoa(rngEnd)
	return &VfGroup{
		ResourceName: p.Spec.ResourceName,
		DeviceType:   p.Spec.DeviceType,
		VfRange:      rng,
		PolicyName:   p.GetName(),
		Mtu:          p.Spec.Mtu,
		IsRdma:       p.Spec.IsRdma,
		VdpaType:     p.Spec.VdpaType,
	}, nil
}

func IndexInRange(i int, r string) bool {
	rngSt, rngEnd, err := parseRange(r)
	if err != nil {
		return false
	}
	if i <= rngEnd && i >= rngSt {
		return true
	}
	return false
}

func parseRange(r string) (rngSt, rngEnd int, err error) {
	rng := strings.Split(r, "-")
	rngSt, err = strconv.Atoi(rng[0])
	if err != nil {
		return
	}
	rngEnd, err = strconv.Atoi(rng[1])
	if err != nil {
		return
	}
	return
}

// SplitDeviceFromRange return the device name and the range.
// the split is base on #
func SplitDeviceFromRange(device string) (string, string) {
	if strings.Contains(device, "#") {
		fields := strings.Split(device, "#")
		return fields[0], fields[1]
	}

	return device, ""
}

// ParseVfRange: parse a device with VF range
// this can be rootDevices or PFName
// if no range detect we just return the device name
func ParseVfRange(device string) (rootDeviceName string, rngSt, rngEnd int, err error) {
	rngSt, rngEnd = invalidVfIndex, invalidVfIndex
	rootDeviceName, splitRange := SplitDeviceFromRange(device)
	if splitRange != "" {
		rngSt, rngEnd, err = parseRange(splitRange)
	} else {
		rootDeviceName = device
	}
	return
}

// IsEmpty returns true if nicSelector is empty
func (selector *SriovNetworkNicSelector) IsEmpty() bool {
	return selector.Vendor == "" &&
		selector.DeviceID == "" &&
		len(selector.RootDevices) == 0 &&
		len(selector.PfNames) == 0 &&
		len(selector.NetFilter) == 0
}

func (selector *SriovNetworkNicSelector) Selected(iface *InterfaceExt) bool {
	if selector.Vendor != "" && selector.Vendor != iface.Vendor {
		return false
	}
	if selector.DeviceID != "" && selector.DeviceID != iface.DeviceID {
		return false
	}
	if len(selector.RootDevices) > 0 && !StringInArray(iface.PciAddress, selector.RootDevices) {
		return false
	}
	if len(selector.PfNames) > 0 {
		var pfNames []string
		for _, p := range selector.PfNames {
			if strings.Contains(p, "#") {
				fields := strings.Split(p, "#")
				pfNames = append(pfNames, fields[0])
			} else {
				pfNames = append(pfNames, p)
			}
		}
		if !StringInArray(iface.Name, pfNames) {
			return false
		}
	}
	if selector.NetFilter != "" && !NetFilterMatch(selector.NetFilter, iface.NetFilter) {
		return false
	}

	return true
}

func (s *SriovNetworkNodeState) GetInterfaceStateByPciAddress(addr string) *InterfaceExt {
	for _, iface := range s.Status.Interfaces {
		if addr == iface.PciAddress {
			return &iface
		}
	}
	return nil
}

func (s *SriovNetworkNodeState) GetDriverByPciAddress(addr string) string {
	for _, iface := range s.Status.Interfaces {
		if addr == iface.PciAddress {
			return iface.Driver
		}
	}
	return ""
}

// RenderNetAttDef renders a net-att-def for ib-sriov CNI
func (cr *SriovIBNetwork) RenderNetAttDef() (*uns.Unstructured, error) {
	logger := log.WithName("RenderNetAttDef")
	logger.Info("Start to render IB SRIOV CNI NetworkAttachmentDefinition")

	// render RawCNIConfig manifests
	data := render.MakeRenderData()
	data.Data["CniType"] = "ib-sriov"
	data.Data["SriovNetworkName"] = cr.Name
	if cr.Spec.NetworkNamespace == "" {
		data.Data["SriovNetworkNamespace"] = cr.Namespace
	} else {
		data.Data["SriovNetworkNamespace"] = cr.Spec.NetworkNamespace
	}
	data.Data["SriovCniResourceName"] = os.Getenv("RESOURCE_PREFIX") + "/" + cr.Spec.ResourceName

	data.Data["StateConfigured"] = true
	switch cr.Spec.LinkState {
	case SriovCniStateEnable:
		data.Data["SriovCniState"] = SriovCniStateEnable
	case SriovCniStateDisable:
		data.Data["SriovCniState"] = SriovCniStateDisable
	case SriovCniStateAuto:
		data.Data["SriovCniState"] = SriovCniStateAuto
	default:
		data.Data["StateConfigured"] = false
	}

	if cr.Spec.Capabilities == "" {
		data.Data["CapabilitiesConfigured"] = false
	} else {
		data.Data["CapabilitiesConfigured"] = true
		data.Data["SriovCniCapabilities"] = cr.Spec.Capabilities
	}

	if cr.Spec.IPAM != "" {
		data.Data["SriovCniIpam"] = SriovCniIpam + ":" + strings.Join(strings.Fields(cr.Spec.IPAM), "")
	} else {
		data.Data["SriovCniIpam"] = SriovCniIpamEmpty
	}

	// metaplugins for the infiniband cni
	data.Data["MetaPluginsConfigured"] = false
	if cr.Spec.MetaPluginsConfig != "" {
		data.Data["MetaPluginsConfigured"] = true
		data.Data["MetaPlugins"] = cr.Spec.MetaPluginsConfig
	}

	// logLevel and logFile are currently not supports by the ip-sriov-cni -> hardcode them to false.
	data.Data["LogLevelConfigured"] = false
	data.Data["LogFileConfigured"] = false

	objs, err := render.RenderDir(filepath.Join(ManifestsPath, "sriov"), &data)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		raw, _ := json.Marshal(obj)
		logger.Info("render NetworkAttachmentDefinition output", "raw", string(raw))
	}
	return objs[0], nil
}

// NetworkNamespace returns target network namespace for the network
func (cr *SriovIBNetwork) NetworkNamespace() string {
	return cr.Spec.NetworkNamespace
}

// RenderNetAttDef renders a net-att-def for sriov CNI
func (cr *SriovNetwork) RenderNetAttDef() (*uns.Unstructured, error) {
	logger := log.WithName("RenderNetAttDef")
	logger.Info("Start to render SRIOV CNI NetworkAttachmentDefinition")

	// render RawCNIConfig manifests
	data := render.MakeRenderData()
	data.Data["CniType"] = "sriov"
	data.Data["SriovNetworkName"] = cr.Name
	if cr.Spec.NetworkNamespace == "" {
		data.Data["SriovNetworkNamespace"] = cr.Namespace
	} else {
		data.Data["SriovNetworkNamespace"] = cr.Spec.NetworkNamespace
	}
	data.Data["SriovCniResourceName"] = os.Getenv("RESOURCE_PREFIX") + "/" + cr.Spec.ResourceName
	data.Data["SriovCniVlan"] = cr.Spec.Vlan

	if cr.Spec.VlanQoS <= 7 && cr.Spec.VlanQoS >= 0 {
		data.Data["VlanQoSConfigured"] = true
		data.Data["SriovCniVlanQoS"] = cr.Spec.VlanQoS
	} else {
		data.Data["VlanQoSConfigured"] = false
	}

	data.Data["VlanProtoConfigured"] = false
	if cr.Spec.VlanProto != "" {
		data.Data["VlanProtoConfigured"] = true
		data.Data["SriovCniVlanProto"] = cr.Spec.VlanProto
	}

	if cr.Spec.Capabilities == "" {
		data.Data["CapabilitiesConfigured"] = false
	} else {
		data.Data["CapabilitiesConfigured"] = true
		data.Data["SriovCniCapabilities"] = cr.Spec.Capabilities
	}

	data.Data["SpoofChkConfigured"] = true
	switch cr.Spec.SpoofChk {
	case SriovCniStateOff:
		data.Data["SriovCniSpoofChk"] = SriovCniStateOff
	case SriovCniStateOn:
		data.Data["SriovCniSpoofChk"] = SriovCniStateOn
	default:
		data.Data["SpoofChkConfigured"] = false
	}

	data.Data["TrustConfigured"] = true
	switch cr.Spec.Trust {
	case SriovCniStateOn:
		data.Data["SriovCniTrust"] = SriovCniStateOn
	case SriovCniStateOff:
		data.Data["SriovCniTrust"] = SriovCniStateOff
	default:
		data.Data["TrustConfigured"] = false
	}

	data.Data["StateConfigured"] = true
	switch cr.Spec.LinkState {
	case SriovCniStateEnable:
		data.Data["SriovCniState"] = SriovCniStateEnable
	case SriovCniStateDisable:
		data.Data["SriovCniState"] = SriovCniStateDisable
	case SriovCniStateAuto:
		data.Data["SriovCniState"] = SriovCniStateAuto
	default:
		data.Data["StateConfigured"] = false
	}

	data.Data["MinTxRateConfigured"] = false
	if cr.Spec.MinTxRate != nil {
		if *cr.Spec.MinTxRate >= 0 {
			data.Data["MinTxRateConfigured"] = true
			data.Data["SriovCniMinTxRate"] = *cr.Spec.MinTxRate
		}
	}

	data.Data["MaxTxRateConfigured"] = false
	if cr.Spec.MaxTxRate != nil {
		if *cr.Spec.MaxTxRate >= 0 {
			data.Data["MaxTxRateConfigured"] = true
			data.Data["SriovCniMaxTxRate"] = *cr.Spec.MaxTxRate
		}
	}

	if cr.Spec.IPAM != "" {
		data.Data["SriovCniIpam"] = SriovCniIpam + ":" + strings.Join(strings.Fields(cr.Spec.IPAM), "")
	} else {
		data.Data["SriovCniIpam"] = SriovCniIpamEmpty
	}

	data.Data["MetaPluginsConfigured"] = false
	if cr.Spec.MetaPluginsConfig != "" {
		data.Data["MetaPluginsConfigured"] = true
		data.Data["MetaPlugins"] = cr.Spec.MetaPluginsConfig
	}

	data.Data["LogLevelConfigured"] = (cr.Spec.LogLevel != "")
	data.Data["LogLevel"] = cr.Spec.LogLevel
	data.Data["LogFileConfigured"] = (cr.Spec.LogFile != "")
	data.Data["LogFile"] = cr.Spec.LogFile

	objs, err := render.RenderDir(filepath.Join(ManifestsPath, "sriov"), &data)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		raw, _ := json.Marshal(obj)
		logger.Info("render NetworkAttachmentDefinition output", "raw", string(raw))
	}
	return objs[0], nil
}

// NetworkNamespace returns target network namespace for the network
func (cr *SriovNetwork) NetworkNamespace() string {
	return cr.Spec.NetworkNamespace
}

// RenderNetAttDef renders a net-att-def for sriov CNI
func (cr *OVSNetwork) RenderNetAttDef() (*uns.Unstructured, error) {
	logger := log.WithName("RenderNetAttDef")
	logger.Info("Start to render OVS CNI NetworkAttachmentDefinition")

	// render RawCNIConfig manifests
	data := render.MakeRenderData()
	data.Data["CniType"] = "ovs"
	data.Data["NetworkName"] = cr.Name
	if cr.Spec.NetworkNamespace == "" {
		data.Data["NetworkNamespace"] = cr.Namespace
	} else {
		data.Data["NetworkNamespace"] = cr.Spec.NetworkNamespace
	}
	data.Data["CniResourceName"] = os.Getenv("RESOURCE_PREFIX") + "/" + cr.Spec.ResourceName

	if cr.Spec.Capabilities == "" {
		data.Data["CapabilitiesConfigured"] = false
	} else {
		data.Data["CapabilitiesConfigured"] = true
		data.Data["CniCapabilities"] = cr.Spec.Capabilities
	}

	data.Data["Bridge"] = cr.Spec.Bridge
	data.Data["VlanTag"] = cr.Spec.Vlan
	data.Data["MTU"] = cr.Spec.MTU
	if len(cr.Spec.Trunk) > 0 {
		trunkConfRaw, _ := json.Marshal(cr.Spec.Trunk)
		data.Data["Trunk"] = string(trunkConfRaw)
	} else {
		data.Data["Trunk"] = ""
	}
	data.Data["InterfaceType"] = cr.Spec.InterfaceType

	if cr.Spec.IPAM != "" {
		data.Data["CniIpam"] = SriovCniIpam + ":" + strings.Join(strings.Fields(cr.Spec.IPAM), "")
	} else {
		data.Data["CniIpam"] = SriovCniIpamEmpty
	}

	data.Data["MetaPluginsConfigured"] = false
	if cr.Spec.MetaPluginsConfig != "" {
		data.Data["MetaPluginsConfigured"] = true
		data.Data["MetaPlugins"] = cr.Spec.MetaPluginsConfig
	}

	objs, err := render.RenderDir(filepath.Join(ManifestsPath, "ovs"), &data)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		raw, _ := json.Marshal(obj)
		logger.Info("render NetworkAttachmentDefinition output", "raw", string(raw))
	}
	return objs[0], nil
}

// NetworkNamespace returns target network namespace for the network
func (cr *OVSNetwork) NetworkNamespace() string {
	return cr.Spec.NetworkNamespace
}

// NetFilterMatch -- parse netFilter and check for a match
func NetFilterMatch(netFilter string, netValue string) (isMatch bool) {
	logger := log.WithName("NetFilterMatch")

	var re = regexp.MustCompile(`(?m)^\s*([^\s]+)\s*:\s*([^\s]+)`)

	netFilterResult := re.FindAllStringSubmatch(netFilter, -1)

	if netFilterResult == nil {
		logger.Info("Invalid NetFilter spec...", "netFilter", netFilter)
		return false
	}

	netValueResult := re.FindAllStringSubmatch(netValue, -1)

	if netValueResult == nil {
		logger.Info("Invalid netValue...", "netValue", netValue)
		return false
	}

	return netFilterResult[0][1] == netValueResult[0][1] && netFilterResult[0][2] == netValueResult[0][2]
}

// MaxUnavailable calculate the max number of unavailable nodes to represent the number of nodes
// we can drain in parallel
func (s *SriovNetworkPoolConfig) MaxUnavailable(numOfNodes int) (int, error) {
	// this means we want to drain all the nodes in parallel
	if s.Spec.MaxUnavailable == nil {
		return -1, nil
	}
	intOrPercent := *s.Spec.MaxUnavailable

	if intOrPercent.Type == intstrutil.String {
		if strings.HasSuffix(intOrPercent.StrVal, "%") {
			i := strings.TrimSuffix(intOrPercent.StrVal, "%")
			v, err := strconv.Atoi(i)
			if err != nil {
				return 0, fmt.Errorf("invalid value %q: %v", intOrPercent.StrVal, err)
			}
			if v > 100 || v < 1 {
				return 0, fmt.Errorf("invalid value: percentage needs to be between 1 and 100")
			}
		} else {
			return 0, fmt.Errorf("invalid type: strings needs to be a percentage")
		}
	}

	maxunavail, err := intstrutil.GetScaledValueFromIntOrPercent(&intOrPercent, numOfNodes, false)
	if err != nil {
		return 0, err
	}

	if maxunavail < 0 {
		return 0, fmt.Errorf("negative number is not allowed")
	}

	return maxunavail, nil
}

// GenerateBridgeName generate predictable name for the software bridge
// current format is: br-0000_00_03.0
func GenerateBridgeName(iface *InterfaceExt) string {
	return fmt.Sprintf("br-%s", strings.ReplaceAll(iface.PciAddress, ":", "_"))
}

// NeedToUpdateBridges returns true if bridge for the host requires update
func NeedToUpdateBridges(bridgeSpec, bridgeStatus *Bridges) bool {
	return !equality.Semantic.DeepEqual(bridgeSpec, bridgeStatus)
}

// SetKeepUntilTime sets an annotation to hold the "keep until time" for the node’s state.
// The "keep until time" specifies the earliest time at which the state object can be removed
// if the daemon's pod is not found on the node.
func (s *SriovNetworkNodeState) SetKeepUntilTime(t time.Time) {
	ts := t.Format(time.RFC3339)
	annotations := s.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[consts.NodeStateKeepUntilAnnotation] = ts
	s.SetAnnotations(annotations)
}

// GetKeepUntilTime returns the value that is stored in the "keep until time" annotation.
// The "keep until time" specifies the earliest time at which the state object can be removed
// if the daemon's pod is not found on the node.
// Return zero time instant if annotaion is not found on the object or if it has a wrong format.
func (s *SriovNetworkNodeState) GetKeepUntilTime() time.Time {
	t, err := time.Parse(time.RFC3339, s.GetAnnotations()[consts.NodeStateKeepUntilAnnotation])
	if err != nil {
		return time.Time{}
	}
	return t
}

// ResetKeepUntilTime removes "keep until time" annotation from the state object.
// The "keep until time" specifies the earliest time at which the state object can be removed
// if the daemon's pod is not found on the node.
// Returns true if the value was removed, false otherwise.
func (s *SriovNetworkNodeState) ResetKeepUntilTime() bool {
	annotations := s.GetAnnotations()
	_, exist := annotations[consts.NodeStateKeepUntilAnnotation]
	if !exist {
		return false
	}
	delete(annotations, consts.NodeStateKeepUntilAnnotation)
	s.SetAnnotations(annotations)
	return true
}
