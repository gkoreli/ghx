package ghx

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

type fakeGithubClientProvider struct {
	graphQL graphQLDoer
	rest    restGetter
}

func (f fakeGithubClientProvider) GraphQL() (graphQLDoer, error) {
	if f.graphQL == nil {
		return nil, fmt.Errorf("unexpected GraphQL client request")
	}
	return f.graphQL, nil
}

func (f fakeGithubClientProvider) REST() (restGetter, error) {
	if f.rest == nil {
		return nil, fmt.Errorf("unexpected REST client request")
	}
	return f.rest, nil
}

func (f fakeGithubClientProvider) RESTWithOptions(api.ClientOptions) (restGetter, error) {
	if f.rest == nil {
		return nil, fmt.Errorf("unexpected REST client request")
	}
	return f.rest, nil
}

type fakeGraphQLClient struct {
	t       *testing.T
	want    []string
	payload string
	calls   int
}

func (f *fakeGraphQLClient) Do(query string, variables map[string]interface{}, response interface{}) error {
	f.calls++
	if variables != nil {
		f.t.Fatalf("variables = %#v, want nil", variables)
	}
	for _, want := range f.want {
		if !strings.Contains(query, want) {
			f.t.Fatalf("query does not contain %q:\n%s", want, query)
		}
	}
	return json.Unmarshal([]byte(f.payload), response)
}

func TestExploreUsesInjectedGraphQLClientOffline(t *testing.T) {
	fakeGQL := &fakeGraphQLClient{
		t:    t,
		want: []string{`repository(owner: "cli", name: "cli")`, `HEAD:README.md`},
		payload: `{
			"repository": {
				"defaultBranchRef": { "name": "trunk" },
				"description": "GitHub CLI",
				"tree": {
					"entries": [
						{ "name": "cmd", "type": "tree" },
						{ "name": "README.md", "type": "blob" }
					]
				},
				"readme1": { "text": "# gh\nprimary readme" },
				"readme2": { "text": "# gh\nlowercase readme" },
				"readme3": { "text": "" }
			}
		}`,
	}

	oldClients := githubClients
	githubClients = fakeGithubClientProvider{graphQL: fakeGQL}
	t.Cleanup(func() { githubClients = oldClients })

	got, err := Explore(Repo{Owner: "cli", Name: "cli"}, "")
	if err != nil {
		t.Fatalf("Explore returned error: %v", err)
	}

	if fakeGQL.calls != 1 {
		t.Fatalf("GraphQL calls = %d, want 1", fakeGQL.calls)
	}
	if got.Description != "GitHub CLI" {
		t.Fatalf("Description = %q, want GitHub CLI", got.Description)
	}
	if got.Branch != "trunk" {
		t.Fatalf("Branch = %q, want trunk", got.Branch)
	}
	if got.Readme != "# gh\nprimary readme" {
		t.Fatalf("Readme = %q, want primary README text", got.Readme)
	}
	wantFiles := []FileEntry{{Name: "cmd", Type: "tree"}, {Name: "README.md", Type: "blob"}}
	if len(got.Files) != len(wantFiles) {
		t.Fatalf("Files = %#v, want %#v", got.Files, wantFiles)
	}
	for i := range wantFiles {
		if got.Files[i] != wantFiles[i] {
			t.Fatalf("Files[%d] = %#v, want %#v", i, got.Files[i], wantFiles[i])
		}
	}
}
