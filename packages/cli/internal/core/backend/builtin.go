package backend

func builtinSpecs() []BackendSpec {
	return []BackendSpec{
		envDotenvSpec(),
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

func envDotenvSpec() BackendSpec {
	return spec(
		BackendID{Domain: DomainEnv, Name: EnvDotenv},
		[]Capability{CapabilityEnvGet, CapabilityEnvSet, CapabilityEnvList, CapabilityEnvInject, CapabilityScaffold},
	)
}

func envInfisicalSpec() BackendSpec {
	return spec(
		BackendID{Domain: DomainEnv, Name: EnvInfisical},
		[]Capability{CapabilityEnvGet, CapabilityEnvSet, CapabilityEnvDelete, CapabilityEnvList, CapabilityEnvPull, CapabilityEnvInject, CapabilityScaffold},
	)
}
