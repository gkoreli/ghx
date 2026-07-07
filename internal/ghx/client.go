package ghx

import "github.com/cli/go-gh/v2/pkg/api"

type graphQLDoer interface {
	Do(query string, variables map[string]interface{}, response interface{}) error
}

type restGetter interface {
	Get(path string, resp interface{}) error
}

type githubClientProvider interface {
	GraphQL() (graphQLDoer, error)
	REST() (restGetter, error)
	RESTWithOptions(opts api.ClientOptions) (restGetter, error)
}

type defaultGithubClientProvider struct{}

func (defaultGithubClientProvider) GraphQL() (graphQLDoer, error) {
	return api.DefaultGraphQLClient()
}

func (defaultGithubClientProvider) REST() (restGetter, error) {
	return api.DefaultRESTClient()
}

func (defaultGithubClientProvider) RESTWithOptions(opts api.ClientOptions) (restGetter, error) {
	return api.NewRESTClient(opts)
}

var githubClients githubClientProvider = defaultGithubClientProvider{}
