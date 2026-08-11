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
	"sigs.k8s.io/controller-runtime/pkg/log"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	mlx "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vendors/mellanox"
)

const (
	VendorID = "15b3"

	// lagResourceAllocation is the mlxconfig parameter controlling SR-IOV
	// multiport (LAG) resource allocation; not yet in nic-configuration-operator consts.
	lagResourceAllocation = "LAG_RESOURCE_ALLOCATION"

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

// requiredNvConfigParams are always supported; queried as a batch.
var requiredNvConfigParams = []string{
	nicconsts.SriovNumOfVfsParam,
	nicconsts.SriovEnabledParam,
	lagResourceAllocation,
}

// optionalNvConfigParams may not be supported on all devices (e.g. ConnectX-6 Dx
// does not support LINK_TYPE_P1/P2); queried individually with errors suppressed.
var optionalNvConfigParams = []string{
	nicconsts.LinkTypeP1Param,
	nicconsts.LinkTypeP2Param,
}

// GetNicFwData queries NV config params via mlxconfig and returns current and
// next-boot state as MlxNic structs compatible with the mlx.Handle* family.
// Optional params (link type) are queried individually; unsupported ones are skipped.
func (h *nvidiaHelper) GetNicFwData(ctx context.Context, pciAddr string) (current, nextBoot *mlx.MlxNic, err error) {
	log.Log.V(2).Info("nvidia GetNicFwData", "pciAddr", pciAddr)

	port := nicv1alpha1.NicDevicePortSpec{PCI: pciAddr}

	query, err := h.nvUtils.QueryNvConfig(ctx, port, requiredNvConfigParams)
	if err != nil {
		return nil, nil, fmt.Errorf("QueryNvConfig for %s: %w", pciAddr, err)
	}

	for _, param := range optionalNvConfigParams {
		q, qErr := h.nvUtils.QueryNvConfig(ctx, port, []string{param})
		if qErr != nil {
			log.Log.V(2).Info("nvidia GetNicFwData: skipping unsupported param",
				"pciAddr", pciAddr, "param", param, "err", qErr)
			continue
		}
		for k, v := range q.CurrentConfig {
			query.CurrentConfig[k] = v
		}
		for k, v := range q.NextBootConfig {
			query.NextBootConfig[k] = v
		}
	}

	current, err = mlxNicFromConfig(query.CurrentConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing current config for %s: %w", pciAddr, err)
	}
	nextBoot, err = mlxNicFromConfig(query.NextBootConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing next boot config for %s: %w", pciAddr, err)
	}
	return current, nextBoot, nil
}

// ApplyNicFwChanges applies only the fields that differ from sentinel values
// (TotalVfs == -1 means skip, empty string means skip) via a single
// dmsc mlxconfig set call through nvUtils.SetNvConfigParametersBatch.
func (h *nvidiaHelper) ApplyNicFwChanges(ctx context.Context, pciAddr string, changes mlx.MlxNic) error {
	_ = ctx
	log.Log.V(2).Info("nvidia ApplyNicFwChanges", "pciAddr", pciAddr)

	params := map[string]string{}

	if changes.EnableSriov {
		params[nicconsts.SriovEnabledParam] = "True"
	} else if changes.TotalVfs == 0 {
		params[nicconsts.SriovEnabledParam] = "False"
	}

	if changes.TotalVfs > -1 {
		params[nicconsts.SriovNumOfVfsParam] = strconv.Itoa(changes.TotalVfs)
	}

	if changes.LinkTypeP1 != "" {
		params[nicconsts.LinkTypeP1Param] = changes.LinkTypeP1
	}

	if changes.LinkTypeP2 != "" {
		params[nicconsts.LinkTypeP2Param] = changes.LinkTypeP2
	}

	if changes.Multiport != -1 {
		params[lagResourceAllocation] = strconv.Itoa(changes.Multiport)
	}

	if len(params) == 0 {
		return nil
	}

	port := nicv1alpha1.NicDevicePortSpec{PCI: pciAddr}
	return h.nvUtils.SetNvConfigParametersBatch(port, params, false, false)
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

// mlxNicFromConfig converts an NvConfigQuery config map (CurrentConfig or
// NextBootConfig) into an MlxNic struct compatible with the mlx.Handle* family.
func mlxNicFromConfig(config map[string][]string) (*mlx.MlxNic, error) {
	nic := &mlx.MlxNic{TotalVfs: 0, Multiport: -1}

	if vals := config[nicconsts.SriovNumOfVfsParam]; len(vals) > 0 {
		v, err := strconv.Atoi(vals[0])
		if err != nil {
			return nil, fmt.Errorf("parse %s %q: %w", nicconsts.SriovNumOfVfsParam, vals[0], err)
		}
		nic.TotalVfs = v
	}

	if vals := config[nicconsts.SriovEnabledParam]; len(vals) > 0 {
		nic.EnableSriov = strings.EqualFold(vals[0], "true") || vals[0] == "1"
	}

	if vals := config[nicconsts.LinkTypeP1Param]; len(vals) > 0 {
		nic.LinkTypeP1 = parseLinkType(vals[0])
	}

	if vals := config[nicconsts.LinkTypeP2Param]; len(vals) > 0 {
		nic.LinkTypeP2 = parseLinkType(vals[0])
	}

	// LAG_RESOURCE_ALLOCATION may be absent on NICs that don't support it.
	if vals := config[lagResourceAllocation]; len(vals) > 0 {
		joined := strings.Join(vals, "")
		if strings.Contains(joined, "1") {
			nic.Multiport = 1
		} else if strings.Contains(joined, "0") {
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
