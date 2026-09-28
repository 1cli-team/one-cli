package backend

func builtinSpecs() []BackendSpec {
	return []BackendSpec{
		envInfisicalSpec(),
	}
}

func spec(id BackendID, capabilities []Capability, requirements ...Requirement) BackendSpec {
	return BackendSpec{
		ID:           id,
		Pair:         id.String(),
		Capabilities: capabilities,
		Requirements: requirements,
	}
}

func envInfisicalSpec() BackendSpec {
	return spec(
		BackendID{Domain: DomainEnv, Name: EnvInfisical},
		[]Capability{CapabilityEnvGet, CapabilityEnvSet, CapabilityEnvDelete, CapabilityEnvList, CapabilityEnvInject, CapabilityScaffold},
	)
}
