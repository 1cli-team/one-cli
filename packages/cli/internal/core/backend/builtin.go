package backend

func builtinSpecs() []BackendSpec {
	return []BackendSpec{
		envDotenvSpec(),
		envInfisicalSpec(),
		s3Spec(DeployAliyunOSS, "https://oss-<region>.aliyuncs.com", "cn-hangzhou", false),
		s3Spec(DeployTencentCOS, "https://cos.<region>.myqcloud.com", "ap-guangzhou", false),
		s3Spec(DeployAWSS3, "", "us-east-1", false),
		s3Spec(DeployMinIO, "http://<your-minio-host>:9000", "us-east-1", true),
		s3Spec(DeployRustFS, "http://<your-rustfs-host>:9000", "us-east-1", true),
		s3Spec(DeployR2, "https://<account-id>.r2.cloudflarestorage.com", "auto", false),
		deployKustomizeSpec(),
		deployVercelSpec(),
		deployCloudflareSpec(),
		deployEdgeOneSpec(),
		containerSpec(ContainerDocker),
		containerSpec(ContainerDockerHub),
		containerSpec(ContainerGHCR),
		containerSpec(ContainerACR),
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
		[]Capability{CapabilityEnvGet, CapabilityEnvSet, CapabilityEnvList, CapabilityEnvPull, CapabilityEnvInject, CapabilityScaffold},
		ProfileSpec{Configurable: true, Type: ProfileTypeInfisical, Fields: fields},
		Requirement{Kind: RequirementProfile, Name: "env/infisical"},
	)
}

func deployKustomizeSpec() BackendSpec {
	fields := []FieldSpec{
		field("kubeconfigPath", "kubeconfig", FieldString, "form.fields.kubeconfigPath", false),
		field("kubeconfigContext", "kubeconfig-context", FieldString, "form.fields.kubeconfigContext", false),
	}
	fields[0].Placeholder = "~/.kube/config"
	return spec(
		BackendID{Domain: DomainDeploy, Name: DeployKustomize},
		[]Capability{CapabilityDeploy, CapabilityScaffold},
		ProfileSpec{Configurable: true, Type: ProfileTypeKustomize, Fields: fields},
		Requirement{Kind: RequirementBinary, Name: "kubectl"},
		Requirement{Kind: RequirementCapability, Name: string(CapabilityContainerBuild)},
		Requirement{Kind: RequirementCapability, Name: string(CapabilityContainerPush)},
	)
}

func s3Spec(name, endpoint, region string, pathStyle bool) BackendSpec {
	fields := []FieldSpec{
		field("endpoint", "endpoint", FieldString, "form.fields.endpoint", false),
		field("region", "region", FieldString, "form.fields.region", false),
		field("forcePathStyle", "force-path-style", FieldBoolean, "form.fields.forcePathStyle", false),
		field("credentials/accessKeyId", "access-key-id", FieldString, "form.fields.accessKeyId", true),
		field("credentials/accessKeySecret", "access-key-secret", FieldSecret, "form.fields.accessKeySecret", true),
	}
	fields[0].Placeholder = endpoint
	fields[1].Default = region
	fields[1].Placeholder = region
	fields[2].Default = pathStyle
	result := spec(
		BackendID{Domain: DomainDeploy, Name: name},
		[]Capability{CapabilityDeploy},
		ProfileSpec{Configurable: true, Type: ProfileTypeS3, Fields: fields},
		Requirement{Kind: RequirementProfile, Name: "deploy/" + name},
	)
	result.Traits = []Trait{TraitS3Compatible}
	return result
}

func deployVercelSpec() BackendSpec {
	return spec(
		BackendID{Domain: DomainDeploy, Name: DeployVercel},
		[]Capability{CapabilityDeploy, CapabilityScaffold},
		ProfileSpec{Configurable: true, Type: ProfileTypeVercel, Fields: []FieldSpec{
			field("team", "team", FieldString, "form.fields.teamSlug", false),
			field("credentials/apiToken", "token", FieldSecret, "form.fields.apiToken", true),
		}},
		Requirement{Kind: RequirementProfile, Name: "deploy/vercel"},
		Requirement{Kind: RequirementBinary, Name: "vercel"},
	)
}

func deployCloudflareSpec() BackendSpec {
	return spec(
		BackendID{Domain: DomainDeploy, Name: DeployCloudflare},
		[]Capability{CapabilityDeploy, CapabilityScaffold},
		ProfileSpec{Configurable: true, Type: ProfileTypeCloudflare, Fields: []FieldSpec{
			field("accountId", "account-id", FieldString, "form.fields.accountId", false),
			field("credentials/apiToken", "token", FieldSecret, "form.fields.apiToken", true),
		}},
		Requirement{Kind: RequirementProfile, Name: "deploy/cloudflare"},
		Requirement{Kind: RequirementBinary, Name: "wrangler"},
	)
}

func deployEdgeOneSpec() BackendSpec {
	return spec(
		BackendID{Domain: DomainDeploy, Name: DeployEdgeOne},
		[]Capability{CapabilityDeploy, CapabilityScaffold},
		ProfileSpec{Configurable: true, Type: ProfileTypeEdgeOne, Fields: []FieldSpec{
			field("region", "region", FieldString, "form.fields.regionEdgeOne", false),
			field("credentials/apiToken", "token", FieldSecret, "form.fields.apiToken", false),
		}},
		Requirement{Kind: RequirementProfile, Name: "deploy/edgeone"},
		Requirement{Kind: RequirementBinary, Name: "edgeone"},
	)
}

func containerSpec(name string) BackendSpec {
	fields := make([]FieldSpec, 0, 5)
	if name == ContainerDocker {
		entry := field("registry", "registry", FieldString, "form.fields.registry", true)
		entry.Placeholder = "your-registry-host"
		fields = append(fields, entry)
	}
	if name == ContainerACR {
		entry := field("region", "region", FieldString, "form.fields.acrRegion", true)
		entry.Default = "cn-hangzhou"
		entry.Placeholder = "cn-hangzhou"
		fields = append(fields, entry)
	}
	fields = append(fields,
		field("credentials/username", "username", FieldString, "form.fields.username", true),
		field("namespace", "namespace", FieldString, "form.fields.namespace", false),
		field("credentials/password", "password", FieldSecret, "form.fields.password", true),
	)
	result := spec(
		BackendID{Domain: DomainContainer, Name: name},
		[]Capability{CapabilityContainerInfo, CapabilityContainerBuild, CapabilityContainerPush, CapabilityScaffold},
		ProfileSpec{Configurable: true, Type: ProfileTypeContainer, Fields: fields},
		Requirement{Kind: RequirementProfile, Name: "container/" + name, Optional: true},
		Requirement{Kind: RequirementBinary, Name: "docker"},
	)
	result.Traits = []Trait{TraitOCIRegistry}
	return result
}
