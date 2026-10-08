package app

const lowMemoryThreshold = uint64(4 << 30)

func resolveLowMemory(mode MemoryMode) bool {
	switch mode {
	case MemoryLow:
		return true
	case MemoryNormal:
		return false
	case MemoryAuto, "":
		total, err := totalRAMBytes()
		return err == nil && total > 0 && total <= lowMemoryThreshold
	default:
		return false
	}
}

// LowMemoryFor is kept small and deterministic for callers/tests that need to
// explain or preview Auto mode without querying the current machine.
func LowMemoryFor(mode MemoryMode, totalRAM uint64) bool {
	if mode == MemoryLow {
		return true
	}
	if mode != MemoryAuto && mode != "" {
		return false
	}
	return totalRAM > 0 && totalRAM <= lowMemoryThreshold
}
