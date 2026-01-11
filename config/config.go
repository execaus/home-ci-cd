package config

import "gopkg.in/yaml.v3"

type RepositoryType string
type CredentialType string

const (
	GithubType RepositoryType = "github"
)

const (
	CredentialSSHType CredentialType = "ssh"
)

type Config struct {
	Credentials Credential `yaml:"credentials"`
	Git         Git        `yaml:"git"`
	// Directory where repositories will be cloned for further processing
	BufferDirectory string `yaml:"bufferDirectory"`
	// Repositories with automation scripts
	Repositories []Repository `yaml:"repositories"`
}

type Repository struct {
	Type RepositoryType `yaml:"type"`
	// Repository owner
	Owner string `yaml:"owner"`
	// Repository name
	Repo string `yaml:"repo"`
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

type Credential struct {
	Type CredentialType `yaml:"type"`
	Data any            `yaml:"data"`
}

func (c Credential) CredentialSSH() (CredentialSSH, error) {
	var zero CredentialSSH

	if c.Type != CredentialSSHType {
		return zero, ErrInvalidCredentialType
	}

	b, err := yaml.Marshal(c.Data)
	if err != nil {
		return zero, err
	}

	var cred CredentialSSH
	if err = yaml.Unmarshal(b, &cred); err != nil {
		return zero, err
	}

	return cred, nil
}

type CredentialSSH struct {
	User       string `yaml:"user"`
	PrivateKey string `yaml:"privateKey"`
	Passphrase string `yaml:"passphrase,omitempty"`
}
