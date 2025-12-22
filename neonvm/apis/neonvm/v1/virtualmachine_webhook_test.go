package v1

import (
	"testing"

	"github.com/samber/lo"
	"github.com/tychoish/fun/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestFieldsAllowedToChangeFromNilOnly(t *testing.T) {
	t.Run("should allow change from nil values", func(t *testing.T) {
		defaultVm := &VirtualMachine{}
		// defaultVm.Default() returns an object with nil value for fields we are interested in
		defaultVm.Default()

		fromNilFields := []struct {
			setter func(*VirtualMachine)
			field  string
		}{
			{
				setter: func(vm *VirtualMachine) {
					vm.Spec.CpuScalingMode = lo.ToPtr(CpuScalingModeQMP)
				},
				field: ".spec.cpuScalingMode",
			},
			{
				setter: func(vm *VirtualMachine) {
					vm.Spec.TargetArchitecture = lo.ToPtr(CPUArchitectureAMD64)
				},
				field: ".spec.targetArchitecture",
			},
		}

		for _, field := range fromNilFields {
			vm2 := defaultVm.DeepCopy()
			field.setter(vm2)
			_, err := vm2.ValidateUpdate(defaultVm)
			assert.NotError(t, err)
		}
	})

	t.Run("should not allow change from non-nil values", func(t *testing.T) {
		defaultVm := &VirtualMachine{}
		defaultVm.Default()
		// override nil values with non-nil values
		defaultVm.Spec.CpuScalingMode = lo.ToPtr(CpuScalingModeQMP)
		defaultVm.Spec.TargetArchitecture = lo.ToPtr(CPUArchitectureAMD64)

		fromNilFields := []struct {
			setter func(*VirtualMachine)
			field  string
		}{
			{
				setter: func(vm *VirtualMachine) {
					vm.Spec.CpuScalingMode = lo.ToPtr(CpuScalingModeSysfs)
				},
				field: ".spec.cpuScalingMode",
			},
			{
				setter: func(vm *VirtualMachine) {
					vm.Spec.TargetArchitecture = lo.ToPtr(CPUArchitectureARM64)
				},
				field: ".spec.targetArchitecture",
			},
		}

		for _, field := range fromNilFields {
			vm2 := defaultVm.DeepCopy()
			field.setter(vm2)
			_, err := vm2.ValidateUpdate(defaultVm)
			assert.Error(t, err)
		}
	})
}
func TestVirtualMachine_Default(t *testing.T) {
	t.Run("should set CPUs.Limit to Max if zero", func(t *testing.T) {
		vm := &VirtualMachine{
			Spec: VirtualMachineSpec{
				Guest: Guest{
					CPUs: CPUs{
						Min: 1000,
						Max: 4000,
						Use: 1000,
					},
					MemorySlots: MemorySlots{
						Min: 1,
						Max: 4,
						Use: 1,
					},
					MemorySlotSize: resource.MustParse("1Gi"),
				},
			},
		}
		vm.Default()
		assert.Equal(t, uint32(vm.Spec.Guest.CPUs.Limit), uint32(4000))
		assert.Equal(t, vm.Spec.Guest.MemorySlots.Limit, int32(4))
	})

	t.Run("should populate PodResources if missing", func(t *testing.T) {
		vm := &VirtualMachine{
			Spec: VirtualMachineSpec{
				Guest: Guest{
					CPUs: CPUs{
						Min: 1000,
						Max: 4000,
						Use: 1000,
					},
					MemorySlots: MemorySlots{
						Min: 1,
						Max: 4,
						Use: 1,
					},
					MemorySlotSize: resource.MustParse("1Gi"),
				},
			},
		}
		vm.Default()
		assert.True(t, len(vm.Spec.PodResources.Limits) > 0)
		assert.Equal(t, vm.Spec.PodResources.Limits[corev1.ResourceCPU], *resource.NewMilliQuantity(4000, resource.BinarySI))
		assert.Equal(t, vm.Spec.PodResources.Limits[corev1.ResourceMemory], *resource.NewQuantity(4*1024*1024*1024, resource.BinarySI))
	})
}

func TestVirtualMachine_ValidateCreate_MemorySlotsLimit(t *testing.T) {
	vm := &VirtualMachine{
		Spec: VirtualMachineSpec{
			Guest: Guest{
				CPUs: CPUs{
					Min:   1000,
					Max:   4000,
					Use:   1000,
					Limit: 4000,
				},
				MemorySlots: MemorySlots{
					Min:   4,
					Max:   8,
					Use:   6,
					Limit: 4,
				},
				MemorySlotSize: resource.MustParse("1Gi"),
			},
		},
	}
	_, err := vm.ValidateCreate()
	assert.NotError(t, err)

	vm.Spec.Guest.MemorySlots.Limit = 3
	_, err = vm.ValidateCreate()
	assert.Error(t, err)

	vm.Spec.Guest.MemorySlots.Limit = 5
	vm.Spec.Guest.MemorySlots.Use = 6
	_, err = vm.ValidateCreate()
	assert.Error(t, err)
}

func TestVirtualMachine_ValidateUpdate_MemorySlotsLimitImmutable(t *testing.T) {
	oldVM := &VirtualMachine{
		Spec: VirtualMachineSpec{
			Guest: Guest{
				CPUs: CPUs{
					Min:   1000,
					Max:   4000,
					Use:   1000,
					Limit: 4000,
				},
				MemorySlots: MemorySlots{
					Min:   4,
					Max:   8,
					Use:   4,
					Limit: 6,
				},
				MemorySlotSize: resource.MustParse("1Gi"),
			},
		},
	}
	newVM := oldVM.DeepCopy()
	newVM.Spec.Guest.MemorySlots.Limit = 7
	_, err := newVM.ValidateUpdate(oldVM)
	assert.Error(t, err)
}
