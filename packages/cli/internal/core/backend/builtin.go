package backend

func builtinSpecs() []BackendSpec {
	return []BackendSpec{
		envDotenvSpec(),
		envInfisicalSpec(),
	}
}

func spec(id BackendID, capabilities []Capability, profile ProfileSpec, requirements ...Requirement) BackendSpec {
	return BackendSpec{
		ID:           id,
		Pair:         id.String(),
		Capabilities: capabilities,
		Requirements: requirements,
		Profile:      profile,
	}
}

func field(path, inputName string, kind FieldType, label string, required bool) FieldSpec {
	return FieldSpec{Path: path, InputName: inputName, Type: kind, LabelKey: label, Required: required}
}

func envDotenvSpec() BackendSpec {
	return spec(
		BackendID{Domain: DomainEnv, Name: EnvDotenv},
		[]Capability{CapabilityEnvGet, CapabilityEnvSet, CapabilityEnvList, CapabilityEnvInject, CapabilityScaffold},
		ProfileSpec{Type: ProfileTypeDotenv},
	)
}

func envInfisicalSpec() BackendSpec {
	fields := []FieldSpec{
		field("siteUrl", "site-url", FieldString, "form.fields.siteUrl", false),
		field("credentials/clientId", "client-id", FieldString, "form.fields.clientId", true),
		field("credentials/clientSecret", "client-secret", FieldSecret, "form.fields.clientSecret", true),
	}
	fields[0].Default = "https://app.infisical.com"
	fields[0].Placeholder = "https://infisical.company.com"
	return spec(
		BackendID{Domain: DomainEnv, Name: EnvInfisical},
		[]Capability{CapabilityEnvGet, CapabilityEnvSet, CapabilityEnvDelete, CapabilityEnvList, CapabilityEnvPull, CapabilityEnvInject, CapabilityScaffold},
		ProfileSpec{Configurable: true, Type: ProfileTypeInfisical, Fields: fields},
		Requirement{Kind: RequirementProfile, Name: "env/infisical"},
	)
}
