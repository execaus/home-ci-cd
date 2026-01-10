package config

type Config struct {
	Git Git `yaml:"git"`
	// Directory where repositories will be cloned for further processing
	BufferDirectory string `yaml:"bufferDirectory"`
	// Repositories with automation scripts
	Repositories []Repository `yaml:"repositories"`
}

type Repository struct {
	// Repository owner
	Owner string `yaml:"owner"`
	// Repository name
	RepoName string `yaml:"repoName"`
	// Automation pipelines for branch processing
	BranchPipelines []BranchPipeline `yaml:"branchPipelines"`
}

type BranchPipeline struct {
	// Branch matching template
	Template string `yaml:"template"`
	// Docker build executable file
	DockerFilePath string `yaml:"dockerFilePath"`
	// Commands to run on remote server
	RemoteCommands []string `yaml:"remoteCommands"`
}

type Git struct {
	// Git related configurations
	Github Github `yaml:"github"`
}

type Github struct {
	// GitHub authentication token
	Token string `yaml:"token"`
}
