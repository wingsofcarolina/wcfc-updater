package github_api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v55/github"
)

type Session struct {
	client *github.Client
}

type AuthParams struct {
	AppID          string
	InstallationID string
	PrivateKey     string
}

// authTransport injects the installation token into all requests
type authTransport struct {
	token string
	base  http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "token "+t.token)
	return t.base.RoundTrip(req)
}

// getInstallationToken exchanges a JWT for an installation token
func getInstallationToken(ctx context.Context, jwtString, installID string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", installID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+jwtString)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	return tokenResp.Token, nil
}

func NewSession(ctx context.Context, auth *AuthParams) (*Session, error) {
	block, _ := pem.Decode([]byte(auth.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block containing the private key: invalid PEM format")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS1 private key: %w", err)
	}

	now := time.Now()
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Unix() - 60,
		"exp": now.Unix() + (9 * 60), // valid for ~10 minutes
		"iss": auth.AppID,
	})
	jwtString, err := jwtToken.SignedString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to sign JWT token with private key: %w", err)
	}

	token, err := getInstallationToken(ctx, jwtString, auth.InstallationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub installation token: %w", err)
	}

	return &Session{
		client: github.NewClient(&http.Client{
			Transport: &authTransport{token: token, base: http.DefaultTransport},
		}),
	}, nil

}

func (s *Session) GetLatestTag(ctx context.Context, owner, repo string) (*github.RepositoryTag, error) {
	refs, _, err := s.client.Git.ListMatchingRefs(ctx, owner, repo, &github.ReferenceListOptions{Ref: "tags/"})
	if err != nil {
		return nil, fmt.Errorf("listing refs: %w", err)
	}

	type tagWithDate struct {
		tag  *github.RepositoryTag
		date time.Time
	}
	var tags []tagWithDate

	for _, r := range refs {
		name := strings.TrimPrefix(r.GetRef(), "refs/tags/")
		sha := r.GetObject().GetSHA()

		// If this is an annotated tag, resolve to the commit object
		if r.GetObject().GetType() == "tag" {
			tobj, _, err := s.client.Git.GetTag(ctx, owner, repo, sha)
			if err != nil || tobj.GetObject() == nil || tobj.GetObject().SHA == nil {
				continue
			}
			sha = tobj.GetObject().GetSHA()
		}

		c, _, err := s.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
		if err != nil || c == nil || c.Commit == nil || c.Commit.Committer == nil || c.Commit.Committer.Date == nil {
			continue
		}

		date := c.GetCommit().GetCommitter().GetDate().Time

		tags = append(tags, tagWithDate{
			tag: &github.RepositoryTag{
				Name:   github.String(name),
				Commit: &github.Commit{SHA: github.String(sha)},
			},
			date: date,
		})
	}

	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags found")
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].date.After(tags[j].date)
	})

	return tags[0].tag, nil
}

type LatestCommits struct {
	LatestTagName string
	LatestTagSHA  string
	Commits       []*github.RepositoryCommit
}

func (s *Session) GetLatestCommits(ctx context.Context, owner string, repo string) (*LatestCommits, error) {
	latestTag, err := s.GetLatestTag(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("error getting latest tag: %w", err)
	}
	tagSHA := latestTag.GetCommit().GetSHA()

	tagCommit, _, err := s.client.Repositories.GetCommit(ctx, owner, repo, tagSHA, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting commit for tag %s: %w", latestTag.GetName(), err)
	}
	tagDate := tagCommit.GetCommit().GetCommitter().GetDate()

	commits, _, err := s.client.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
		Since:       tagDate.Time,
		ListOptions: github.ListOptions{PerPage: 50},
	})
	if err != nil {
		return nil, fmt.Errorf("error getting commits for tag %s: %w", latestTag.GetName(), err)
	}

	return &LatestCommits{
		LatestTagName: latestTag.GetName(),
		LatestTagSHA:  tagSHA,
		Commits:       commits,
	}, nil

}

func (s *Session) RunWorkflowDispatch(ctx context.Context, owner string, repo string,
	ref string, workflowFile string, inputs map[string]any) error {
	req := github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs,
	}
	_, err := s.client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, workflowFile, req)
	if err != nil {
		return fmt.Errorf("failed to trigger workflow: %w", err)
	}
	return nil
}
