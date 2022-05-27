package common

import (
	"github.com/jaypipes/ghw/pkg/gpu"
	"github.com/jaypipes/ghw/pkg/pci"
)

type StubHardware struct {
	GPU []*gpu.GraphicsCard
	PCI []*pci.Device
}

func (hw *StubHardware) getGPU() ([]*gpu.GraphicsCard, error) {
	return hw.GPU, nil
}

func (hw *StubHardware) getPCI() ([]*pci.Device, error) {
	return hw.PCI, nil
}
