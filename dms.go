package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mellanox/nic-configuration-operator/api/v1alpha1"
	"github.com/Mellanox/nic-configuration-operator/pkg/consts"
	"github.com/Mellanox/nic-configuration-operator/pkg/dms"
	"github.com/Mellanox/nic-configuration-operator/pkg/nvconfig"
)

const (
	targetSerial = "MT2116X00001"
	targetPCI    = "0000:3b:00.0"
)

// mlxParams are the four NV config parameters the mellanox plugin reads and
// may modify: total VF count, SR-IOV enable, and per-port link type.
var mlxParams = []string{
	consts.SriovNumOfVfsParam, // NUM_OF_VFS
	consts.SriovEnabledParam,  // SRIOV_EN
	consts.LinkTypeP1Param,    // LINK_TYPE_P1
	consts.LinkTypeP2Param,    // LINK_TYPE_P2
}

func main() {
	ctx := context.Background()

	deviceStatus := v1alpha1.NicDeviceStatus{
		SerialNumber: targetSerial,
		Ports: []v1alpha1.NicDevicePortSpec{
			{PCI: targetPCI, NetworkInterface: "eth0"},
		},
	}

	// Start the local DMS server (manages dmsd processes, one per device).
	dmsMgr := dms.NewDMSManager()
	if err := dmsMgr.StartDMSInstances([]v1alpha1.NicDeviceStatus{deviceStatus}); err != nil {
		log.Fatalf("start DMS instances: %v", err)
	}
	defer dmsMgr.StopAllDMSInstances()

	// Query all NV config in one mlxconfig call (empty additionalParameter = full dump).
	nvUtils := nvconfig.NewNVConfigUtils()
	query, err := nvUtils.QueryNvConfig(ctx, targetPCI, "")
	if err != nil {
		log.Fatalf("QueryNvConfig: %v", err)
	}

	for _, param := range mlxParams {
		vals, ok := query.CurrentConfig[param]
		if !ok || len(vals) == 0 {
			fmt.Printf("NIC %s  %s = <not found>\n", targetPCI, param)
			continue
		}
		fmt.Printf("NIC %s  %s = %s\n", targetPCI, param, vals[0])
	}

	// MTU is not an mlxconfig/DMS parameter; read it from sysfs.
	iface := deviceStatus.Ports[0].NetworkInterface
	mtuRaw, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "mtu"))
	if err != nil {
		log.Fatalf("read MTU for %s: %v", iface, err)
	}
	fmt.Printf("NIC %s  MTU = %s\n", targetPCI, strings.TrimSpace(string(mtuRaw)))
}
