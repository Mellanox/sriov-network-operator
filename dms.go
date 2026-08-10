package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Mellanox/nic-configuration-operator/api/v1alpha1"
	"github.com/Mellanox/nic-configuration-operator/pkg/consts"
	"github.com/Mellanox/nic-configuration-operator/pkg/dms"
	"github.com/Mellanox/nic-configuration-operator/pkg/nvconfig"
)

const (
	targetSerial = "MT2116X00001"
	targetPCI    = "0000:3b:00.0"
)

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

	// NUM_OF_VFS is an NV config (mlxconfig) parameter, not a DMS gNMI path.
	// Query it via NVConfigUtils; pass consts.SriovNumOfVfsParam to fetch only that param.
	nvUtils := nvconfig.NewNVConfigUtils()
	query, err := nvUtils.QueryNvConfig(ctx, targetPCI, consts.SriovNumOfVfsParam)
	if err != nil {
		log.Fatalf("QueryNvConfig: %v", err)
	}

	vals, ok := query.CurrentConfig[consts.SriovNumOfVfsParam]
	if !ok || len(vals) == 0 {
		log.Fatalf("%s not found in current config", consts.SriovNumOfVfsParam)
	}

	fmt.Printf("NIC %s  %s = %s\n", targetPCI, consts.SriovNumOfVfsParam, vals[0])
}
