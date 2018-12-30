package vsphere

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"

	"github.com/vmware/govmomi/vim25/types"
)

// VirtualMachine virtual machine wrapper
type VirtualMachine struct {
	Ref       types.ManagedObjectReference
	Name      string
	Datastore *Datastore
}

// GuestInfos the guest infos
// Must not start with `guestinfo.`
type GuestInfos map[string]string

type extraConfig []types.BaseOptionValue

func (g GuestInfos) isEmpty() bool {
	return len(g) == 0
}

func (g GuestInfos) toExtraConfig() extraConfig {
	extraConfig := make(extraConfig, 0, len(g))

	for k, v := range g {
		extraConfig.Set(fmt.Sprintf("guestinfo.%s", k), v)
	}

	return extraConfig
}

func (e *extraConfig) String() string {
	return fmt.Sprintf("%v", *e)
}

func (e *extraConfig) Set(k, v string) {
	*e = append(*e, &types.OptionValue{Key: k, Value: v})
}

// VirtualMachine return govmomi virtual machine
func (vm *VirtualMachine) VirtualMachine(ctx *Context) *object.VirtualMachine {
	key := vm.Ref.String()

	if v := ctx.Value(key); v != nil {
		return v.(*object.VirtualMachine)
	}

	f := vm.Datastore.Datacenter.NewFinder(ctx)

	v, err := f.ObjectReference(ctx, vm.Ref)

	if err != nil {
		glog.Fatalf("Can't find virtual machine:%s", vm.Name)
	}

	//	v := object.NewVirtualMachine(vm.VimClient(), vm.Ref)

	ctx.WithValue(key, v)
	ctx.WithValue(fmt.Sprintf("[%s] %s", vm.Datastore.Name, vm.Name), v)

	return v.(*object.VirtualMachine)
}

// VimClient return the VIM25 client
func (vm *VirtualMachine) VimClient() *vim25.Client {
	return vm.Datastore.VimClient()
}

// Configure set characteristic of VM a virtual machine
func (vm *VirtualMachine) Configure(ctx *Context, guestInfos *GuestInfos, annotation string, memory int, cpus int, disk int) error {
	var err error
	var task *object.Task

	if cpus > 0 || memory > 0 || len(annotation) > 0 {
		virtualMachine := vm.VirtualMachine(ctx)

		vmConfigSpec := types.VirtualMachineConfigSpec{}

		if cpus > 0 {
			vmConfigSpec.NumCPUs = int32(cpus)
		}

		if memory > 0 {
			vmConfigSpec.MemoryMB = int64(memory)
		}

		vmConfigSpec.Annotation = annotation

		if guestInfos != nil && guestInfos.isEmpty() == false {
			vmConfigSpec.ExtraConfig = guestInfos.toExtraConfig()
		}

		if task, err = virtualMachine.Reconfigure(ctx, vmConfigSpec); err == nil {
			_, err = task.WaitForResult(ctx, nil)
		}
	}

	return nil
}

// WaitForIP wait ip
func (vm *VirtualMachine) WaitForIP(ctx *Context) (string, error) {
	var powerState types.VirtualMachinePowerState
	var err error
	var ip string

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			ip, err = v.WaitForIP(ctx)
		} else {
			err = fmt.Errorf("The VM: %s is not powered", v.InventoryPath)
		}
	}

	return ip, err
}

// PowerOn power on a virtual machine
func (vm *VirtualMachine) PowerOn(ctx *Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState != types.VirtualMachinePowerStatePoweredOn {
			task, err = v.PowerOn(ctx)

			_, err = task.WaitForResult(ctx, nil)
		} else {
			err = fmt.Errorf("The VM: %s is already powered", v.InventoryPath)
		}
	}

	return err
}

// PowerOff power off a virtual machine
func (vm *VirtualMachine) PowerOff(ctx *Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState == types.VirtualMachinePowerStatePoweredOn {
			task, err = v.PowerOff(ctx)

			_, err = task.WaitForResult(ctx, nil)
		} else {
			err = fmt.Errorf("The VM: %s is already power off", v.InventoryPath)
		}
	}

	return err
}

// Delete delete the virtual machine
func (vm *VirtualMachine) Delete(ctx *Context) error {
	var powerState types.VirtualMachinePowerState
	var err error
	var task *object.Task

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		if powerState != types.VirtualMachinePowerStatePoweredOn {
			task, err = v.Destroy(ctx)

			_, err = task.WaitForResult(ctx, nil)
		} else {
			err = fmt.Errorf("The VM: %s is powered", v.InventoryPath)
		}
	}

	return err
}

// Status refresh status virtual machine
func (vm *VirtualMachine) Status(ctx *Context) (*Status, error) {
	var powerState types.VirtualMachinePowerState
	var err error
	var status *Status

	v := vm.VirtualMachine(ctx)

	if powerState, err = v.PowerState(ctx); err == nil {
		address := ""

		if powerState == types.VirtualMachinePowerStatePoweredOn {
			address, err = v.WaitForIP(ctx)
		}

		status = &Status{
			Address: address,
			Powered: powerState == types.VirtualMachinePowerStatePoweredOn,
		}
	}

	return status, err
}

// SetGuestInfo change guest ingos
func (vm *VirtualMachine) SetGuestInfo(ctx *Context, guestInfos *GuestInfos) error {
	var task *object.Task
	var err error

	vmConfigSpec := types.VirtualMachineConfigSpec{}
	v := vm.VirtualMachine(ctx)

	if guestInfos != nil && guestInfos.isEmpty() == false {
		vmConfigSpec.ExtraConfig = guestInfos.toExtraConfig()
	} else {
		vmConfigSpec.ExtraConfig = []types.BaseOptionValue{}
	}

	if task, err = v.Reconfigure(ctx, vmConfigSpec); err == nil {
		err = task.Wait(ctx)
	}

	return err
}
