package nvidia

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	sriovnetworkv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/consts"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/helper"
	plugin "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins"
	mellanoxplugin "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/plugins/mellanox"
	"github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vars"
	mlx "github.com/k8snetworkplumbingwg/sriov-network-operator/pkg/vendors/mellanox"
)

var PluginName = "NvidiaPlugin"

type NvidiaPlugin struct {
	PluginName          string
	helpers             helper.HostHelpersInterface
	pciAddressesToReset []string
	attributesToChange  map[string]mlx.MlxNic
	nicsStatus          map[string]map[string]sriovnetworkv1.InterfaceExt
	nicsSpec            map[string]sriovnetworkv1.Interface
}

func NewNvidiaPlugin(helpers helper.HostHelpersInterface) (plugin.VendorPlugin, error) {
	if !vars.UsingSystemdMode {
		if err := mellanoxplugin.EnsureDDIStaged(); err != nil {
			log.Log.Error(err, "NvidiaPlugin: failed to stage DDI assets; DDI will be skipped at runtime")
		}
	}
	helpers.SetVFConfigHook(mellanoxplugin.NewMellanoxVFHook(helpers, helpers))

	return &NvidiaPlugin{
		PluginName:          PluginName,
		helpers:             helpers,
		pciAddressesToReset: []string{},
		attributesToChange:  map[string]mlx.MlxNic{},
		nicsStatus:          map[string]map[string]sriovnetworkv1.InterfaceExt{},
		nicsSpec:            map[string]sriovnetworkv1.Interface{},
	}, nil
}

func (p *NvidiaPlugin) Name() string {
	return p.PluginName
}

// OnNodeStateChange is invoked when SriovNetworkNodeState CR is created or updated.
// It mirrors the mellanox plugin logic: collecting NIC status/spec, handling
// TotalVfs, SR-IOV enable, eswitch params, and link type changes.
func (p *NvidiaPlugin) OnNodeStateChange(new *sriovnetworkv1.SriovNetworkNodeState) (needDrain bool, needReboot bool, err error) {
	log.Log.Info("nvidia plugin OnNodeStateChange()")

	p.pciAddressesToReset = []string{}
	p.attributesToChange = map[string]mlx.MlxNic{}
	p.nicsStatus = map[string]map[string]sriovnetworkv1.InterfaceExt{}
	p.nicsSpec = map[string]sriovnetworkv1.Interface{}
	processedNics := map[string]bool{}

	// Collect all Nvidia NIC statuses grouped by PCI prefix (physical NIC).
	for _, iface := range new.Status.Interfaces {
		if iface.Vendor != mlx.MellanoxVendorID {
			continue
		}
		pciPrefix := mlx.GetPciAddressPrefix(iface.PciAddress)
		if ifaces, ok := p.nicsStatus[pciPrefix]; ok {
			ifaces[iface.PciAddress] = iface
		} else {
			p.nicsStatus[pciPrefix] = map[string]sriovnetworkv1.InterfaceExt{iface.PciAddress: iface}
		}
	}

	// Add only Nvidia NICs that have a desired spec.
	for _, iface := range new.Spec.Interfaces {
		pciPrefix := mlx.GetPciAddressPrefix(iface.PciAddress)
		if _, ok := p.nicsStatus[pciPrefix]; !ok {
			continue
		}
		p.nicsSpec[iface.PciAddress] = iface
	}

	if p.helpers.IsKernelLockdownMode() {
		if len(p.nicsSpec) > 0 {
			log.Log.Info("Lockdown mode detected, failing on interface update for nvidia devices")
			return false, false, fmt.Errorf("nvidia device detected when in lockdown mode")
		}
		log.Log.Info("Lockdown mode detected, skipping nvidia nic processing")
		return
	}

	for _, ifaceSpec := range p.nicsSpec {
		pciPrefix := mlx.GetPciAddressPrefix(ifaceSpec.PciAddress)
		// Skip already-processed NICs (prevents double-processing on dual-port cards).
		if _, ok := processedNics[pciPrefix]; ok {
			continue
		}
		processedNics[pciPrefix] = true

		fwCurrent, fwNext, err := p.helpers.GetMlxNicFwData(ifaceSpec.PciAddress)
		if err != nil {
			return false, false, err
		}

		isDualPort := mlx.IsDualPort(ifaceSpec.PciAddress, p.nicsStatus)
		attrs := &mlx.MlxNic{TotalVfs: -1, Multiport: -1}
		var changeWithoutReboot bool

		totalVfs, totalVfsNeedReboot, totalVfsChangeWithoutReboot := mlx.HandleTotalVfs(fwCurrent, fwNext, attrs, ifaceSpec, isDualPort, p.nicsSpec)
		sriovEnNeedReboot, sriovEnChangeWithoutReboot := mlx.HandleEnableSriov(totalVfs, fwCurrent, fwNext, attrs)
		needReboot = totalVfsNeedReboot || sriovEnNeedReboot
		changeWithoutReboot = totalVfsChangeWithoutReboot || sriovEnChangeWithoutReboot

		needESwitchParamsChange := mlx.HandleESwitchParams(pciPrefix, attrs, fwCurrent, p.nicsSpec, p.nicsStatus)
		needReboot = needReboot || needESwitchParamsChange

		needLinkChange, err := mlx.HandleLinkType(pciPrefix, fwCurrent, attrs, p.nicsSpec, p.nicsStatus)
		if err != nil {
			return false, false, err
		}
		needReboot = needReboot || needLinkChange

		// No FW changes allowed when NIC is externally managed.
		if ifaceSpec.ExternallyManaged {
			if totalVfsNeedReboot || totalVfsChangeWithoutReboot {
				return false, false, fmt.Errorf(
					"interface %s required a change in the TotalVfs but the policy is externally managed failing: firmware TotalVf %d requested TotalVf %d",
					ifaceSpec.PciAddress, fwCurrent.TotalVfs, totalVfs)
			}
			if needLinkChange {
				return false, false, fmt.Errorf("change required for link type but the policy is externally managed, failing")
			}
		}

		if needReboot || changeWithoutReboot {
			p.attributesToChange[ifaceSpec.PciAddress] = *attrs
		}
		if needReboot {
			p.pciAddressesToReset = append(p.pciAddressesToReset, ifaceSpec.PciAddress)
		}
	}

	// Reset TotalVfs to 0 for Nvidia NICs that are present but have no spec.
	for pciPrefix, portsMap := range p.nicsStatus {
		if _, ok := processedNics[pciPrefix]; ok {
			continue
		}
		processedNics[pciPrefix] = true
		pciAddress := pciPrefix + "0"

		isConfigured, err := p.nicConfiguredByOperator(portsMap)
		if err != nil {
			return false, false, err
		}
		if !isConfigured {
			log.Log.V(2).Info("None of the ports are configured by the operator skipping firmware reset",
				"portMap", portsMap)
			continue
		}

		hasExternally, err := p.nicHasExternallyManagedPFs(portsMap)
		if err != nil {
			return false, false, err
		}
		if hasExternally {
			log.Log.V(2).Info("One of the ports is configured as externally managed skipping firmware reset",
				"portMap", portsMap)
			continue
		}

		if id := sriovnetworkv1.GetVfDeviceID(portsMap[pciAddress].DeviceID); id == "" {
			continue
		}

		_, fwNext, err := p.helpers.GetMlxNicFwData(pciAddress)
		if err != nil {
			return false, false, err
		}

		if fwNext.TotalVfs > 0 || fwNext.EnableSriov {
			p.attributesToChange[pciAddress] = mlx.MlxNic{TotalVfs: 0, Multiport: -1}
			log.Log.V(2).Info("Changing TotalVfs to 0, doesn't require rebooting", "fwNext.totalVfs", fwNext.TotalVfs)
		}
	}

	if needReboot {
		needDrain = true
	}
	log.Log.V(2).Info("nvidia plugin", "need-drain", needDrain, "need-reboot", needReboot)
	return
}

// CheckStatusChanges verifies whether SriovNetworkNodeState CR status presents changes on configured VFs.
// TODO: implement - https://github.com/k8snetworkplumbingwg/sriov-network-operator/issues/631
func (p *NvidiaPlugin) CheckStatusChanges(*sriovnetworkv1.SriovNetworkNodeState) (bool, error) {
	return false, nil
}

// Apply applies the firmware configuration changes collected in OnNodeStateChange.
func (p *NvidiaPlugin) Apply() error {
	if p.helpers.IsKernelLockdownMode() {
		log.Log.Info("nvidia plugin Apply() - skipping due to lockdown mode")
		return nil
	}
	log.Log.Info("nvidia plugin Apply()")
	if err := p.helpers.MlxConfigFW(p.attributesToChange); err != nil {
		return err
	}
	if vars.FeatureGate.IsEnabled(consts.MellanoxFirmwareResetFeatureGate) {
		return p.helpers.MlxResetFW(p.pciAddressesToReset, p.nicsStatus)
	}
	return nil
}

func (p *NvidiaPlugin) nicHasExternallyManagedPFs(nicPortsMap map[string]sriovnetworkv1.InterfaceExt) (bool, error) {
	for _, iface := range nicPortsMap {
		pfStatus, exist, err := p.helpers.LoadPfsStatus(iface.PciAddress)
		if err != nil {
			log.Log.Error(err, "failed to load PF status from disk. "+
				"This should not happen, to overcome config daemon stuck, "+
				"please remove the PCI file on the host under the operator configuration path",
				"path", consts.PfAppliedConfig, "pciAddress", iface.PciAddress)
			return false, err
		}
		if !exist {
			continue
		}
		if pfStatus.ExternallyManaged {
			log.Log.V(2).Info("PF is externally managed, skip FW TotalVfs reset")
			return true, nil
		}
	}
	return false, nil
}

func (p *NvidiaPlugin) nicConfiguredByOperator(nicPortsMap map[string]sriovnetworkv1.InterfaceExt) (bool, error) {
	for _, iface := range nicPortsMap {
		_, exist, err := p.helpers.LoadPfsStatus(iface.PciAddress)
		if err != nil {
			log.Log.Error(err, "failed to load PF status from disk. "+
				"This should not happen, to overcome config daemon stuck, "+
				"please remove the PCI file on the host under the operator configuration path",
				"path", consts.PfAppliedConfig, "pciAddress", iface.PciAddress)
			return false, err
		}
		if exist {
			log.Log.V(2).Info("PF configured by the operator", "interface", iface)
			return true, nil
		}
	}
	return false, nil
}
