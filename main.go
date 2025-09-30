package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wingsofcarolina/wcfc-updater/pkg/api_version"
	"github.com/wingsofcarolina/wcfc-updater/pkg/github_api"
)

type Repo struct {
	Name     string `yaml:"name"`
	Hostname string `yaml:"hostname"`
}

type Config struct {
	Owner string `yaml:"owner"`
	Repos []Repo `yaml:"repos"`
}

func getAuth() (*github_api.AuthParams, error) {
	key := os.Getenv("GITHUB_PRIVATE_KEY")
	if key == "" {
		return nil, fmt.Errorf("GITHUB_PRIVATE_KEY environment variable not set")
	}
	if !strings.Contains(key, "PRIVATE KEY") {
		pk, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(os.Getenv("GITHUB_PRIVATE_KEY"), " ", ""))
		if err != nil {
			return nil, fmt.Errorf("GITHUB_PRIVATE_KEY is neither private key nor base64 encoded: %w", err)
		}
		key = string(pk)
	}
	ap := &github_api.AuthParams{
		AppID:          os.Getenv("GITHUB_APP_ID"),
		InstallationID: os.Getenv("GITHUB_INSTALLATION_ID"),
		PrivateKey:     key,
	}
	if ap.AppID == "" {
		return nil, fmt.Errorf("GITHUB_APP_ID environment variable not set")
	}
	if ap.InstallationID == "" {
		return nil, fmt.Errorf("GITHUB_INSTALLATION_ID environment variable not set")
	}
	return ap, nil
}

func getConfig() (*Config, error) {
	fn := os.Getenv("WCFC_UPDATER_CONFIG")
	if fn == "" {
		fn = "config.yml"
	}
	f, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()
	var config Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("error decoding yaml: %v", err)
	}
	return &config, nil
}

func processRepo(ctx context.Context, sess *github_api.Session, owner string, repo Repo) error {
	fmt.Printf("Repo: %s\n", repo.Name)
	defer fmt.Printf("\n")

	// Get the latest commits from the GitHub repo
	lt, err := sess.GetLatestCommits(ctx, owner, repo.Name)
	if err != nil {
		return fmt.Errorf("failed to get latest commits: %w", err)
	}

	// Get commit from /api/version on the running site
	runningCommit, err := api_version.GetVersionCommit(repo.Hostname)
	if err != nil {
		return fmt.Errorf("failed to get version commit: %w", err)
	}

	// Print results (tag commit + later commits)
	fmt.Printf("    Current running version: %s\n", runningCommit[:7])
	fmt.Printf("    Latest tag in repo: %s (%s)\n", lt.LatestTagName, lt.LatestTagSHA[:7])
	fmt.Printf("    Commits from latest tag:\n")
	for _, c := range lt.Commits {
		fmt.Printf("        %s %s %s\n", c.GetSHA()[:7], c.GetAuthor().GetLogin(),
			strings.SplitN(c.GetCommit().GetMessage(), "\n", 2)[0])
	}

	if runningCommit != lt.LatestTagSHA {
		fmt.Printf("Skipping: running commit %s differs from latest tag commit %s\n", runningCommit[:7], lt.LatestTagSHA[:7])
		return nil
	}

	// Remove tag SHA from commits list
	for i, c := range lt.Commits {
		if c.GetSHA() == lt.LatestTagSHA {
			lt.Commits = append(lt.Commits[:i], lt.Commits[i+1:]...)
			break
		}
	}

	if len(lt.Commits) == 0 {
		fmt.Printf("Skipping: no post-tag commits found\n")
		return nil
	}

	for _, commit := range lt.Commits {
		if commit.GetAuthor().GetLogin() != "dependabot[bot]" {
			fmt.Printf("Skipping: non-Dependabot post-tag commits found\n")
			return nil
		}
	}

	fmt.Printf("Dispatching release workflow...\n")
	err = sess.RunWorkflowDispatch(ctx, owner, repo.Name, "main", "release.yml", nil)
	if err != nil {
		return fmt.Errorf("failed to dispatch release workflow: %w", err)
	}

	return nil
}

func mainFunc() error {
	ctx := context.Background()

	ap, err := getAuth()
	if err != nil {
		return fmt.Errorf("failed to get auth: %w", err)
	}

	sess, err := github_api.NewSession(ctx, ap)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	config, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	for _, repo := range config.Repos {
		err := processRepo(ctx, sess, config.Owner, repo)
		if err != nil {
			return fmt.Errorf("failed to process repo: %w", err)
		}
	}

	return nil
}

func main() {
	err := mainFunc()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
