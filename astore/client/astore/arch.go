package astore

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
)

func GuessELF(name string) ([]Arch, error) {
	f, err := elf.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	machine := "unsupported"
	switch f.Machine {
	case elf.EM_ARM:
		machine = "arm"
	case elf.EM_AARCH64:
		machine = "arm64"
	case elf.EM_X86_64:
		machine = "amd64"
	case elf.EM_386:
		machine = "i386"
	}
	return []Arch{Arch{Os: "linux", Cpu: machine}}, nil
}

func GuessMacDwarf(name string) ([]Arch, error) {
	f, err := macho.Open(name)
	if err != nil {
		return nil, err
	}
	machine := MacCpuString(f.Cpu)
	return []Arch{Arch{Os: "mac", Cpu: machine}}, nil
}

func MacCpuString(cpu macho.Cpu) string {
	machine := "unsupported"
	switch cpu {
	case macho.CpuArm:
		machine = "arm"
	case macho.CpuArm64:
		machine = "arm64"
	case macho.CpuAmd64:
		machine = "amd64"
	case macho.Cpu386:
		machine = "i386"
	}
	return machine
}

func GuessMacFat(name string) ([]Arch, error) {
	f, err := macho.OpenFat(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	archs := []Arch{}
	for _, arch := range f.Arches {
		machine := MacCpuString(arch.Cpu)
		archs = append(archs, Arch{Os: "mac", Cpu: machine})
	}
	return archs, nil
}

func GuessPe(name string) ([]Arch, error) {
	f, err := pe.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	machine := "unsupported"
	switch f.Machine {
	case pe.IMAGE_FILE_MACHINE_ARM:
		machine = "arm"
	case pe.IMAGE_FILE_MACHINE_ARM64:
		machine = "arm64"
	case pe.IMAGE_FILE_MACHINE_AMD64:
		machine = "amd64"
	case pe.IMAGE_FILE_MACHINE_I386:
		machine = "i386"
	}
	return []Arch{Arch{Os: "win", Cpu: machine}}, nil
}

type Arch struct {
	Cpu string
	Os  string
}

func (a Arch) ToString() string {
	return a.Cpu + "-" + a.Os
}

func ToArchArray(archs []Arch) []string {
	result := []string{}
	for _, arch := range archs {
		result = append(result, arch.ToString())
	}
	return result
}

func GuessArchOS(name string) ([]Arch, error) {
	errs := []error{}

	names := []string{"elf (linux/unix)", "macho fat (mac)", "macho dwarf (mac)", "pe (windows)"}
	for ix, f := range []func(string) ([]Arch, error){GuessELF, GuessMacDwarf, GuessMacFat, GuessPe} {
		arch, err := f(name)
		if err == nil {
			return arch, nil
		}
		errs = append(errs, fmt.Errorf("%s: %s", names[ix], err))
	}
	return nil, multierror.New(errs)
}
