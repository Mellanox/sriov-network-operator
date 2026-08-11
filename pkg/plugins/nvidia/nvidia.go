package nvidia

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/helper"
	plugin "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins"
	nvidiavendor "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vendors/nvidia"
)

var PluginName = "NvidiaPlugin"

type NvidiaPlugin struct {
	PluginName string
	nvidia     nvidiavendor.NvidiaInterface
	// nicsStatus maps PCI address to interface status for all Nvidia NICs
	// discovered in the last OnNodeStateChange call.
	nicsStatus map[string]sriovnetworkv1.InterfaceExt
}

func NewNvidiaPlugin(_ helper.HostHelpersInterface) (plugin.VendorPlugin, error) {
	return &NvidiaPlugin{
		PluginName: PluginName,
		nvidia:     nvidiavendor.New(),
		nicsStatus: map[string]sriovnetworkv1.InterfaceExt{},
	}, nil
}

func (p *NvidiaPlugin) Name() string {
	return p.PluginName
}

// OnNodeStateChange discovers Nvidia NICs in the node state, starts DMS for
// them, and reads their current NV-config parameters and MTU.
func (p *NvidiaPlugin) OnNodeStateChange(state *sriovnetworkv1.SriovNetworkNodeState) (needDrain bool, needReboot bool, err error) {
	log.Log.Info("nvidia plugin OnNodeStateChange()")

	// Stop any previous DMS instances before rebuilding the device list.
	if stopErr := p.nvidia.StopNicManagement(); stopErr != nil {
		log.Log.V(2).Info("StopNicManagement (ignored on first call)", "err", stopErr)
	}

	p.nicsStatus = map[string]sriovnetworkv1.InterfaceExt{}
	var nvidiaIfaces []sriovnetworkv1.InterfaceExt

	for _, iface := range state.Status.Interfaces {
		if iface.Vendor != nvidiavendor.VendorID {
			continue
		}
		p.nicsStatus[iface.PciAddress] = iface
		nvidiaIfaces = append(nvidiaIfaces, iface)
	}

	if len(nvidiaIfaces) == 0 {
		log.Log.V(2).Info("no Nvidia NICs found in node state")
		return false, false, nil
	}

	if err := p.nvidia.StartNicManagement(nvidiaIfaces); err != nil {
		return false, false, fmt.Errorf("StartNicManagement: %w", err)
	}

	ctx := context.Background()
	for pci, iface := range p.nicsStatus {
		fwData, err := p.nvidia.GetNicFwData(ctx, pci, iface.Name)
		if err != nil {
			log.Log.Error(err, "failed to get NIC fw data", "pci", pci)
			continue
		}
		log.Log.Info("nvidia NIC parameters",
			"pci", pci,
			"NUM_OF_VFS", fwData.TotalVfs,
			"SRIOV_EN", fwData.EnableSriov,
			"LINK_TYPE_P1", fwData.LinkTypeP1,
			"LINK_TYPE_P2", fwData.LinkTypeP2,
			"MTU", fwData.MTU,
		)
	}

	return false, false, nil
}

// CheckStatusChanges is not yet implemented for the nvidia plugin.
func (p *NvidiaPlugin) CheckStatusChanges(*sriovnetworkv1.SriovNetworkNodeState) (bool, error) {
	return false, nil
}

// Apply applies any pending configuration changes for Nvidia NICs.
func (p *NvidiaPlugin) Apply() error {
	log.Log.Info("nvidia plugin Apply()")
	return nil
}
