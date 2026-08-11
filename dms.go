package main

import (
	"context"
	"fmt"
	"log"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	nvidiavendor "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vendors/nvidia"
)

const (
	targetSerial = "MT2116X00001"
	targetPCI    = "0000:3b:00.0"
	targetIface  = "enp59s0f0np0"
)

func main() {
	ctx := context.Background()

	iface := sriovnetworkv1.InterfaceExt{
		PciAddress: targetPCI,
		Name:       targetIface,
		Vendor:     nvidiavendor.VendorID,
	}

	helper := nvidiavendor.New()
	if err := helper.StartNicManagement([]sriovnetworkv1.InterfaceExt{iface}); err != nil {
		log.Fatalf("StartNicManagement: %v", err)
	}
	defer helper.StopNicManagement()

	fwData, err := helper.GetNicFwData(ctx, targetPCI, targetIface)
	if err != nil {
		log.Fatalf("GetNicFwData: %v", err)
	}

	fmt.Printf("NIC %s  NUM_OF_VFS  = %d\n", targetPCI, fwData.TotalVfs)
	fmt.Printf("NIC %s  SRIOV_EN    = %v\n", targetPCI, fwData.EnableSriov)
	fmt.Printf("NIC %s  LINK_TYPE_P1= %s\n", targetPCI, fwData.LinkTypeP1)
	fmt.Printf("NIC %s  LINK_TYPE_P2= %s\n", targetPCI, fwData.LinkTypeP2)
	fmt.Printf("NIC %s  MTU         = %d\n", targetPCI, fwData.MTU)
}
