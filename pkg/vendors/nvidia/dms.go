package nvidia

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	nicv1alpha1 "github.com/Mellanox/nic-configuration-operator/api/v1alpha1"
	nicconsts "github.com/Mellanox/nic-configuration-operator/pkg/consts"
	"github.com/Mellanox/nic-configuration-operator/pkg/dms"
	"github.com/Mellanox/nic-configuration-operator/pkg/nvconfig"
	nnictypes "github.com/Mellanox/nic-configuration-operator/pkg/types"
	nicutils "github.com/Mellanox/nic-configuration-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	mlx "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vendors/mellanox"
)

const (
	VendorID = "15b3"

	// lagResourceAllocation is the mlxconfig parameter controlling SR-IOV
	// multiport (LAG) resource allocation; not yet in nic-configuration-operator consts.
	lagResourceAllocation = "LAG_RESOURCE_ALLOCATION"

	// DMS gNMI paths for the NV config parameters managed by this plugin.
	dmsPathNumVfs           = "/nvidia/sriov/config/num-vfs"
	dmsPathSriovEnable      = "/nvidia/sriov/config/enable"
	dmsPathLinkTypeP1       = "/nvidia/port-config/p1/link-type"
	dmsPathLinkTypeP2       = "/nvidia/port-config/p2/link-type"
	dmsPathLagResourceAlloc = "/nvidia/lag/config/resource-allocation"
)

//go:generate ../../../../bin/mockgen -destination mock/mock_nvidia.go -source dms.go
type NvidiaInterface interface {
	// StartNicManagement starts a local DMS server for the supplied Nvidia interfaces.
	StartNicManagement(ifaces []sriovnetworkv1.InterfaceExt) error
	// StopNicManagement stops the running DMS server.
	StopNicManagement() error
	// GetNicFwData returns the current and next-boot NV config for a NIC as
	// MlxNic structs, for direct use with the mlx.Handle* family of functions.
	GetNicFwData(ctx context.Context, pciAddr string) (current, nextBoot *mlx.MlxNic, err error)
	// ApplyNicFwChanges writes the desired NV config changes to the NIC via nvconfig.
	ApplyNicFwChanges(ctx context.Context, pciAddr string, changes mlx.MlxNic) error
	// ResetNicFirmware resets the NIC's NV config to defaults via nvconfig.
	ResetNicFirmware(pciAddr string) error
	// GetMTU returns the MTU for the given network interface from sysfs.
	GetMTU(iface string) (int, error)
}

type nvidiaHelper struct {
	dmsServer dms.DMSServer
	nvUtils   nvconfig.NVConfigUtils
}

func New() NvidiaInterface {
	return &nvidiaHelper{
		dmsServer: dms.NewDMSServer(),
		nvUtils:   nvconfig.NewNVConfigUtils(),
	}
}

func (h *nvidiaHelper) StartNicManagement(ifaces []sriovnetworkv1.InterfaceExt) error {
	log.Log.V(2).Info("nvidia StartNicManagement", "deviceCount", len(ifaces))
	// dmsd expects one entry per physical NIC (PCI prefix, function stripped).
	// Group ports by prefix so dual-port NICs produce a single NicDevice.
	byPrefix := map[string]*nicv1alpha1.NicDevice{}
	for _, iface := range ifaces {
		prefix := mlx.GetPciAddressPrefix(iface.PciAddress)
		dev, ok := byPrefix[prefix]
		if !ok {
			d := nicDeviceFromIface(iface)
			byPrefix[prefix] = &d
		} else {
			dev.Status.Ports = append(dev.Status.Ports, nicv1alpha1.NicDevicePortSpec{
				PCI:              iface.PciAddress,
				NetworkInterface: iface.Name,
			})
		}
	}
	devices := make([]nicv1alpha1.NicDevice, 0, len(byPrefix))
	for _, dev := range byPrefix {
		devices = append(devices, *dev)
	}
	return h.dmsServer.StartDMSServer(devices)
}

func (h *nvidiaHelper) StopNicManagement() error {
	log.Log.V(2).Info("nvidia StopNicManagement")
	return h.dmsServer.StopDMSServer()
}

// dmsNvConfigParams is the set of ConfigurationParameter descriptors sent to
// DMSClient.GetParameters. Each DMSPath must match what dmsd exposes.
var dmsNvConfigParams = []nnictypes.ConfigurationParameter{
	{Name: nicconsts.SriovNumOfVfsParam, DMSPath: dmsPathNumVfs},
	{Name: nicconsts.SriovEnabledParam, DMSPath: dmsPathSriovEnable},
	{Name: nicconsts.LinkTypeP1Param, DMSPath: dmsPathLinkTypeP1},
	{Name: nicconsts.LinkTypeP2Param, DMSPath: dmsPathLinkTypeP2},
	{Name: lagResourceAllocation, DMSPath: dmsPathLagResourceAlloc},
}

// GetNicFwData queries NV config params via DMS and returns the current device
// state as MlxNic structs compatible with the mlx.Handle* family.
// DMS exposes live (current) state only; nextBoot is set to the same values
// because dmsd has no next-boot concept for NV config parameters.
func (h *nvidiaHelper) GetNicFwData(ctx context.Context, pciAddr string) (current, nextBoot *mlx.MlxNic, err error) {
	_ = ctx
	log.Log.V(2).Info("nvidia GetNicFwData", "pciAddr", pciAddr)

	client, err := h.dmsServer.GetDMSClientByPCIAddress(nicutils.PCIDeviceAddress(pciAddr))
	if err != nil {
		return nil, nil, fmt.Errorf("GetDMSClientByPCIAddress for %s: %w", pciAddr, err)
	}

	values, err := client.GetParameters(dmsNvConfigParams)
	if err != nil {
		return nil, nil, fmt.Errorf("GetParameters for %s: %w", pciAddr, err)
	}

	current, err = mlxNicFromDMSValues(values)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing DMS values for %s: %w", pciAddr, err)
	}
	// DMS returns live state; use it as next-boot too so that mlx.Handle*
	// functions see a consistent view and still detect firmware-level changes.
	nextBoot = current
	return current, nextBoot, nil
}

// ApplyNicFwChanges applies only the fields that differ from sentinel values
// (TotalVfs == -1 means skip, empty string means skip) via DMSClient.SetParameters.
func (h *nvidiaHelper) ApplyNicFwChanges(ctx context.Context, pciAddr string, changes mlx.MlxNic) error {
	_ = ctx
	log.Log.V(2).Info("nvidia ApplyNicFwChanges", "pciAddr", pciAddr)

	var params []nnictypes.ConfigurationParameter

	if changes.EnableSriov {
		params = append(params, nnictypes.ConfigurationParameter{
			Name: nicconsts.SriovEnabledParam, DMSPath: dmsPathSriovEnable,
			Value: "true", ValueType: "bool",
		})
	} else if changes.TotalVfs == 0 {
		params = append(params, nnictypes.ConfigurationParameter{
			Name: nicconsts.SriovEnabledParam, DMSPath: dmsPathSriovEnable,
			Value: "false", ValueType: "bool",
		})
	}

	if changes.TotalVfs > -1 {
		params = append(params, nnictypes.ConfigurationParameter{
			Name: nicconsts.SriovNumOfVfsParam, DMSPath: dmsPathNumVfs,
			Value: strconv.Itoa(changes.TotalVfs), ValueType: "uint",
		})
	}

	if changes.LinkTypeP1 != "" {
		params = append(params, nnictypes.ConfigurationParameter{
			Name: nicconsts.LinkTypeP1Param, DMSPath: dmsPathLinkTypeP1,
			Value: changes.LinkTypeP1, ValueType: "string",
		})
	}

	if changes.LinkTypeP2 != "" {
		params = append(params, nnictypes.ConfigurationParameter{
			Name: nicconsts.LinkTypeP2Param, DMSPath: dmsPathLinkTypeP2,
			Value: changes.LinkTypeP2, ValueType: "string",
		})
	}

	if changes.Multiport != -1 {
		params = append(params, nnictypes.ConfigurationParameter{
			Name: lagResourceAllocation, DMSPath: dmsPathLagResourceAlloc,
			Value: strconv.Itoa(changes.Multiport), ValueType: "uint",
		})
	}

	if len(params) == 0 {
		return nil
	}

	client, err := h.dmsServer.GetDMSClientByPCIAddress(nicutils.PCIDeviceAddress(pciAddr))
	if err != nil {
		return fmt.Errorf("GetDMSClientByPCIAddress for %s: %w", pciAddr, err)
	}
	return client.SetParameters(params)
}

// ResetNicFirmware resets all NV config parameters to factory defaults.
func (h *nvidiaHelper) ResetNicFirmware(pciAddr string) error {
	log.Log.V(2).Info("nvidia ResetNicFirmware", "pciAddr", pciAddr)
	return h.nvUtils.ResetNvConfig(nicv1alpha1.NicDevicePortSpec{PCI: pciAddr})
}

// GetMTU reads the interface MTU from /sys/class/net/<iface>/mtu.
func (h *nvidiaHelper) GetMTU(iface string) (int, error) {
	raw, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "mtu"))
	if err != nil {
		return 0, fmt.Errorf("read MTU for %s: %w", iface, err)
	}
	mtu, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, fmt.Errorf("parse MTU for %s: %w", iface, err)
	}
	return mtu, nil
}

// mlxNicFromDMSValues converts the map[DMSPath]value returned by
// DMSClient.GetParameters into an MlxNic struct.
func mlxNicFromDMSValues(values map[string]string) (*mlx.MlxNic, error) {
	nic := &mlx.MlxNic{TotalVfs: 0, Multiport: -1}

	if v, ok := values[dmsPathNumVfs]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("parse %s %q: %w", dmsPathNumVfs, v, err)
		}
		nic.TotalVfs = n
	}

	if v, ok := values[dmsPathSriovEnable]; ok {
		nic.EnableSriov = strings.EqualFold(v, "true") || v == "1"
	}

	if v, ok := values[dmsPathLinkTypeP1]; ok {
		nic.LinkTypeP1 = parseLinkType(v)
	}

	if v, ok := values[dmsPathLinkTypeP2]; ok {
		nic.LinkTypeP2 = parseLinkType(v)
	}

	// LAG_RESOURCE_ALLOCATION may be absent on NICs that don't support it.
	if v, ok := values[dmsPathLagResourceAlloc]; ok {
		if strings.Contains(v, "1") {
			nic.Multiport = 1
		} else if strings.Contains(v, "0") {
			nic.Multiport = 0
		}
	}

	return nic, nil
}

// parseLinkType normalises an mlxconfig/nvconfig link-type value ("eth", "ETH",
// "ETH(2)", "2", …) to the canonical "ETH" / "IB" strings used by the operator.
func parseLinkType(val string) string {
	upper := strings.ToUpper(val)
	if strings.Contains(upper, "ETH") {
		return "ETH"
	} else if strings.Contains(upper, "IB") {
		return "IB"
	} else if val != "" {
		return mlx.UnknownLinkType
	}
	return mlx.PreconfiguredLinkType
}

// nicDeviceFromIface builds a NicDevice for the DMS server from an InterfaceExt.
func nicDeviceFromIface(iface sriovnetworkv1.InterfaceExt) nicv1alpha1.NicDevice {
	return nicv1alpha1.NicDevice{
		Status: nicv1alpha1.NicDeviceStatus{
			Ports: []nicv1alpha1.NicDevicePortSpec{
				{
					PCI:              iface.PciAddress,
					NetworkInterface: iface.Name,
				},
			},
		},
	}
}
