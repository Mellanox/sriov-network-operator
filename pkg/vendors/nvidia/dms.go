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
)

const VendorID = "15b3"

// NicFwData holds the firmware/NV-config parameters the nvidia plugin reads
// and may modify, mirroring the four fields used by the mellanox plugin.
type NicFwData struct {
	TotalVfs    int
	EnableSriov bool
	LinkTypeP1  string
	LinkTypeP2  string
	MTU         int
}

//go:generate ../../../../bin/mockgen -destination mock/mock_nvidia.go -source dms.go
type NvidiaInterface interface {
	// StartNicManagement starts a local DMS instance for each supplied Nvidia interface.
	StartNicManagement(ifaces []sriovnetworkv1.InterfaceExt) error
	// StopNicManagement stops all running DMS instances.
	StopNicManagement() error
	// GetNicFwData returns the current NV-config parameters and MTU for a NIC.
	GetNicFwData(ctx context.Context, pciAddr, iface string) (*NicFwData, error)
}

type nvidiaHelper struct {
	dmsMgr  dms.DMSManager
	nvUtils nvconfig.NVConfigUtils
}

func New() NvidiaInterface {
	return &nvidiaHelper{
		dmsMgr:  dms.NewDMSManager(),
		nvUtils: nvconfig.NewNVConfigUtils(),
	}
}

func (h *nvidiaHelper) StartNicManagement(ifaces []sriovnetworkv1.InterfaceExt) error {
	log.Log.V(2).Info("nvidia StartNicManagement", "deviceCount", len(ifaces))
	devices := make([]nicv1alpha1.NicDeviceStatus, 0, len(ifaces))
	for _, iface := range ifaces {
		devices = append(devices, nicDeviceStatusFromIface(iface))
	}
	return h.dmsMgr.StartDMSInstances(devices)
}

func (h *nvidiaHelper) StopNicManagement() error {
	log.Log.V(2).Info("nvidia StopNicManagement")
	return h.dmsMgr.StopAllDMSInstances()
}

func (h *nvidiaHelper) GetNicFwData(ctx context.Context, pciAddr, iface string) (*NicFwData, error) {
	log.Log.V(2).Info("nvidia GetNicFwData", "pciAddr", pciAddr, "iface", iface)

	// Query all NV config in one mlxconfig call; empty additionalParameter = full dump.
	query, err := h.nvUtils.QueryNvConfig(ctx, pciAddr, "")
	if err != nil {
		return nil, fmt.Errorf("QueryNvConfig for %s: %w", pciAddr, err)
	}

	data := &NicFwData{}

	if vals := query.CurrentConfig[nicconsts.SriovNumOfVfsParam]; len(vals) > 0 {
		data.TotalVfs, _ = strconv.Atoi(vals[0])
	}
	if vals := query.CurrentConfig[nicconsts.SriovEnabledParam]; len(vals) > 0 {
		data.EnableSriov = vals[0] == "1" || strings.EqualFold(vals[0], "true")
	}
	if vals := query.CurrentConfig[nicconsts.LinkTypeP1Param]; len(vals) > 0 {
		data.LinkTypeP1 = vals[0]
	}
	if vals := query.CurrentConfig[nicconsts.LinkTypeP2Param]; len(vals) > 0 {
		data.LinkTypeP2 = vals[0]
	}

	// MTU is not an mlxconfig/DMS parameter; read it directly from sysfs.
	mtuRaw, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "mtu"))
	if err != nil {
		return nil, fmt.Errorf("read MTU for %s: %w", iface, err)
	}
	data.MTU, _ = strconv.Atoi(strings.TrimSpace(string(mtuRaw)))

	return data, nil
}

// nicDeviceStatusFromIface builds a NicDeviceStatus for the DMS manager from
// an InterfaceExt. The PCI address is used as the serial-number key because
// SriovNetworkNodeState does not expose NIC serial numbers.
func nicDeviceStatusFromIface(iface sriovnetworkv1.InterfaceExt) nicv1alpha1.NicDeviceStatus {
	return nicv1alpha1.NicDeviceStatus{
		SerialNumber: iface.PciAddress,
		Ports: []nicv1alpha1.NicDevicePortSpec{
			{
				PCI:              iface.PciAddress,
				NetworkInterface: iface.Name,
			},
		},
	}
}
