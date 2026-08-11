package main

import (
	"context"
	"fmt"
	"log"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	nvidiavendor "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vendors/nvidia"
)

const (
	targetPCI   = "0000:3b:00.0"
	targetIface = "enp59s0f0np0"
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

	current, _, err := helper.GetNicFwData(ctx, targetPCI)
	if err != nil {
		log.Fatalf("GetNicFwData: %v", err)
	}

	fmt.Printf("NIC %s  NUM_OF_VFS  = %d\n", targetPCI, current.TotalVfs)
	fmt.Printf("NIC %s  SRIOV_EN    = %v\n", targetPCI, current.EnableSriov)
	fmt.Printf("NIC %s  LINK_TYPE_P1= %s\n", targetPCI, current.LinkTypeP1)
	fmt.Printf("NIC %s  LINK_TYPE_P2= %s\n", targetPCI, current.LinkTypeP2)

	mtu, err := helper.GetMTU(targetIface)
	if err != nil {
		log.Fatalf("GetMTU: %v", err)
	}
	fmt.Printf("NIC %s  MTU         = %d\n", targetPCI, mtu)
}
