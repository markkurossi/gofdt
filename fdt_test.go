package gofdt

import (
	"fmt"
	"testing"
)

type MemoryUnit uint64

const (
	_             = iota
	KB MemoryUnit = 1 << (10 * iota)
	MB
	GB
	TB
)

const (
	RamBaseAddr    = 0x80000000
	ClintBaseAddr  = 0x02000000
	ClintSize      = 0x000c0000
	VIRTIOBaseAddr = 0x40010000
	VIRTIOSize     = 0x1000
	VirtualIOIrq   = 1
	PLICBaseAddr   = 0x40100000
	PLICSize       = 0x00400000
	RtcFreq        = 10000000
)

func TestFDT_Workflow(t *testing.T) {
	mem := make([]byte, RamBaseAddr)

	fdt := NewFDT(mem)

	curPHandle := uint32(1)

	fdt.BeginNode("")
	fdt.PropU32("#address-cells", 2)
	fdt.PropU32("#size-cells", 2)
	fdt.PropStr("compatible", "ucbbar,riscvemu-bar_dev")
	fdt.PropStr("model", "ucbbar,riscvemu-bare")

	/* CPU list */
	fdt.BeginNode("cpus")
	fdt.PropU32("#address-cells", 1)
	fdt.PropU32("#size-cells", 0)
	fdt.PropU32("timebase-frequency", RtcFreq)

	/* cpu */
	fdt.BeginNodeNum("cpu", 0)
	fdt.PropStr("device_type", "cpu")
	fdt.PropU32("reg", 0)
	fdt.PropStr("status", "okay")
	fdt.PropStr("compatible", "riscv")

	maxXLen := 128 * MB
	misa := 19
	isaString := fmt.Sprintf("rv%d", maxXLen)
	for i := 0; i < 26; i++ {
		if misa&(1<<i) != 0 {
			isaString += string('a' + byte(i))
		}
	}
	fdt.PropStr("riscv,isa", isaString)
	fdt.PropStr("mmu-type", func() string {
		if maxXLen <= 32 {
			return "riscv,sv32"
		} else {
			return "riscv,sv48"
		}
	}())
	fdt.PropU32("clock-frequency", 2000000000)
	fdt.EndNode() // cpu

	fdt.BeginNode("interrupt-controller")
	fdt.PropU32("#interrupt-cells", 1)
	fdt.Prop("interrupt-controller", nil, 0)
	fdt.PropStr("compatible", "riscv,cpu-intc")
	intCPHandle := curPHandle
	curPHandle++
	fdt.PropU32("phandle", intCPHandle)
	fdt.EndNode() // interrupt-controller

	fdt.EndNode() // cpus

	fdt.BeginNodeNum("memory", RamBaseAddr)
	fdt.PropStr("device_type", "memory")

	kernelStart := uint64(12)
	kernelSize := uint64(1024 * 1024 * 16) // 16MB
	tab := [4]uint32{
		uint32(kernelStart >> 32),
		uint32(kernelStart),
		uint32(kernelStart + kernelSize>>32),
		uint32(kernelStart + kernelSize),
	}
	fdt.PropTabU32("reg", &tab[0], 4)
	fdt.EndNode() // memory

	fdt.BeginNode("htif")
	fdt.PropStr("compatible", "ucb,htif0")
	fdt.EndNode() // htif

	fdt.BeginNode("soc")
	fdt.PropU32("#address-cells", 2)
	fdt.PropU32("#size-cells", 2)
	fdt.PropTabStr("compatible", "ucbbar,riscvemu-bar-soc", "simple-bus")
	//fdt.prop("ranges", nil, 0)

	fdt.BeginNodeNum("clint", ClintBaseAddr)
	fdt.PropStr("compatible", "riscv,clint0")

	tab[0] = intCPHandle
	tab[1] = 3 // M IPI irq
	tab[2] = intCPHandle
	tab[3] = 7 // M timer irq
	fdt.PropTabU32("interrupts-extended", &tab[0], 4)

	fdt.PropTabU64Double("reg", ClintBaseAddr, ClintSize)

	fdt.EndNode() // clint

	fdt.BeginNodeNum("plic", PLICBaseAddr)
	fdt.PropU32("#interrupt-cells", 1)
	fdt.Prop("interrupt-controller", nil, 0)
	fdt.PropStr("compatible", "riscv,plic0")
	fdt.PropU32("riscv,ndev", 31)
	fdt.PropTabU64Double("reg", PLICBaseAddr, PLICSize)
	tab[0] = intCPHandle
	tab[1] = 9 // S ext irq
	tab[2] = intCPHandle
	tab[3] = 11 // M ext irq
	fdt.PropTabU32("interrupts-extended", &tab[0], 4)
	plicPHandle := curPHandle
	curPHandle++
	fdt.PropU32("phandle", plicPHandle)
	fdt.EndNode() // plic

	VIRTIoCount := 3
	for i := 0; i < VIRTIoCount; i++ {
		fdt.BeginNodeNum("virtio", uint64(VIRTIOBaseAddr+i*VIRTIOSize))
		fdt.PropStr("compatible", "virtio,mmio")
		fdt.PropTabU64Double("reg", uint64(VIRTIOBaseAddr+i*VIRTIOSize), VIRTIOSize)
		tab[0] = plicPHandle
		tab[1] = VirtualIOIrq + uint32(i)
		fdt.PropTabU32("interrupts-extended", &tab[0], 2)
		fdt.EndNode() // virtio
	}

	fdt.EndNode() // soc

	fdt.BeginNode("chosen")
	cmdLine := "loglevel=3 console=hvc0 root=/dev/vda rw"
	fdt.PropStr("bootargs", cmdLine)
	if kernelSize > 0 {
		fdt.PropTabU64("riscv,kernel-start", kernelStart)
		fdt.PropTabU64("riscv,kernel-end", kernelStart+kernelSize)
	}

	initrdSize := uint64(30)
	initrdStart := uint64(10)
	if initrdSize > 0 {
		fdt.PropTabU64("linux,initrd-start", initrdStart)
		fdt.PropTabU64("linux,initrd-end", initrdStart+initrdSize)
	}
	fdt.EndNode()
	fdt.EndNode()

	fdt.DumpDTB("./output.dtb")
}
