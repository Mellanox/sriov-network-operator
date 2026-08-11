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

// nvConfigParams are the NV config parameters queried and applied by this package.
var nvConfigParams = []string{
	nicconsts.SriovNumOfVfsParam,
	nicconsts.SriovEnabledParam,
	nicconsts.LinkTypeP1Param,
	nicconsts.LinkTypeP2Param,
	lagResourceAllocation,
}

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
	devices := make([]nicv1alpha1.NicDevice, 0, len(ifaces))
	for _, iface := range ifaces {
		devices = append(devices, nicDeviceFromIface(iface))
	}
	return h.dmsServer.StartDMSServer(devices)
}

func (h *nvidiaHelper) StopNicManagement() error {
	log.Log.V(2).Info("nvidia StopNicManagement")
	return h.dmsServer.StopDMSServer()
}

// GetNicFwData queries the NV config params used by the plugin and returns
// current and next-boot state as MlxNic structs.
func (h *nvidiaHelper) GetNicFwData(ctx context.Context, pciAddr string) (current, nextBoot *mlx.MlxNic, err error) {
	log.Log.V(2).Info("nvidia GetNicFwData", "pciAddr", pciAddr)

	port := nicv1alpha1.NicDevicePortSpec{PCI: pciAddr}
	query, err := h.nvUtils.QueryNvConfig(ctx, port, nvConfigParams)
	if err != nil {
		return nil, nil, fmt.Errorf("QueryNvConfig for %s: %w", pciAddr, err)
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
// (TotalVfs == -1 means skip, empty string means skip) via SetNvConfigParameter.
func (h *nvidiaHelper) ApplyNicFwChanges(ctx context.Context, pciAddr string, changes mlx.MlxNic) error {
	log.Log.V(2).Info("nvidia ApplyNicFwChanges", "pciAddr", pciAddr)
	_ = ctx

	port := nicv1alpha1.NicDevicePortSpec{PCI: pciAddr}

	if changes.EnableSriov {
		if err := h.nvUtils.SetNvConfigParameter(port, nicconsts.SriovEnabledParam, "True"); err != nil {
			return fmt.Errorf("set %s: %w", nicconsts.SriovEnabledParam, err)
		}
	} else if changes.TotalVfs == 0 {
		if err := h.nvUtils.SetNvConfigParameter(port, nicconsts.SriovEnabledParam, "False"); err != nil {
			return fmt.Errorf("set %s: %w", nicconsts.SriovEnabledParam, err)
		}
	}

	if changes.TotalVfs > -1 {
		if err := h.nvUtils.SetNvConfigParameter(port, nicconsts.SriovNumOfVfsParam, strconv.Itoa(changes.TotalVfs)); err != nil {
			return fmt.Errorf("set %s: %w", nicconsts.SriovNumOfVfsParam, err)
		}
	}

	if changes.LinkTypeP1 != "" {
		if err := h.nvUtils.SetNvConfigParameter(port, nicconsts.LinkTypeP1Param, changes.LinkTypeP1); err != nil {
			return fmt.Errorf("set %s: %w", nicconsts.LinkTypeP1Param, err)
		}
	}

	if changes.LinkTypeP2 != "" {
		if err := h.nvUtils.SetNvConfigParameter(port, nicconsts.LinkTypeP2Param, changes.LinkTypeP2); err != nil {
			return fmt.Errorf("set %s: %w", nicconsts.LinkTypeP2Param, err)
		}
	}

	if changes.Multiport != -1 {
		if err := h.nvUtils.SetNvConfigParameter(port, lagResourceAllocation, strconv.Itoa(changes.Multiport)); err != nil {
			return fmt.Errorf("set %s: %w", lagResourceAllocation, err)
		}
	}

	return nil
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
