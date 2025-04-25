package repo

type ProviderType string

const (
	GitHub ProviderType = "github"
	GitLab ProviderType = "gitlab"
)

func (p ProviderType) String() string {
	return string(p)
}

func (p ProviderType) IsValid() bool {
	switch p {
	case GitHub, GitLab:
		return true
	default:
		return false
	}
}
