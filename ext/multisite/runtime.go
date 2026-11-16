package multisite

// pick chooses the runtime for inst: the one it names in Runtime, else the first that serves it.
func pick(runtimes []Runtime, inst Instance) Runtime {
	if inst.Runtime != "" {
		for _, rt := range runtimes {
			if rt.Name() == inst.Runtime {
				return rt
			}
		}
		return nil
	}
	for _, rt := range runtimes {
		if rt.Serves(inst) {
			return rt
		}
	}
	return nil
}
