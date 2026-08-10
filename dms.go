package main

import (
	"fmt"
	"log"

	"github.com/Mellanox/nic-configuration-operator/api/v1alpha1"
	"github.com/Mellanox/nic-configuration-operator/pkg/consts"
	"github.com/Mellanox/nic-configuration-operator/pkg/dms"
	"github.com/Mellanox/nic-configuration-operator/pkg/types"
)

const (
	// Serial number of the target NIC.
	targetSerial = "MT2116X00001"

	// DMS gNMI path for the SR-IOV VF count on a ConnectX NIC.
	sriovNumVfsDMSPath = "/interfaces/interface/nvidia/sriov/config/num-vfs"
)

func main() {
	deviceStatus := v1alpha1.NicDeviceStatus{
		SerialNumber: targetSerial,
		Ports: []v1alpha1.NicDevicePortSpec{
			{PCI: "0000:3b:00.0", NetworkInterface: "eth0"},
		},
	}

	// Initialize the local DMS manager (starts a dmsd process per device).
	dmsMgr := dms.NewDMSManager()
	if err := dmsMgr.StartDMSInstances([]v1alpha1.NicDeviceStatus{deviceStatus}); err != nil {
		log.Fatalf("start DMS instances: %v", err)
	}
	defer dmsMgr.StopAllDMSInstances()

	// Obtain a per-device client identified by serial number.
	dmsClient, err := dmsMgr.GetDMSClientBySerialNumber(targetSerial)
	if err != nil {
		log.Fatalf("get DMS client for %s: %v", targetSerial, err)
	}

	// GetParameters batches all paths into a single dmsc get call per interface.
	// The returned map is keyed by DMSPath (generated selectors stripped).
	params := []types.ConfigurationParameter{
		{
			Name:      consts.SriovNumOfVfsParam, // "NUM_OF_VFS" — label only here
			DMSPath:   sriovNumVfsDMSPath,
			ValueType: "int",
		},
	}

	values, err := dmsClient.GetParameters(params)
	if err != nil {
		log.Fatalf("GetParameters: %v", err)
	}

	numVFs, ok := values[sriovNumVfsDMSPath]
	if !ok {
		log.Fatalf("parameter %s not found in response", sriovNumVfsDMSPath)
	}

	fmt.Printf("NIC %s  %s = %s\n", targetSerial, consts.SriovNumOfVfsParam, numVFs)
}
