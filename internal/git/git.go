package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/alihamzam/git-web-server/config"
	"github.com/alihamzam/git-web-server/pkg"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type zerologWriter struct {
	log zerolog.Logger
}

func (w zerologWriter) Write(p []byte) (n int, err error) {
	w.log.Debug().Msg(string(bytes.TrimSpace(p)))
	return len(p), nil
}

type WebhookMap map[string]chan struct{}

type source struct {
	File     string
	Template bool
}

type gitRepo struct {
	data   *git.Repository
	config *config.Repo
	auth   http.AuthMethod
}

type Git struct {
	filesMap       *pkg.RWMap
	progressWriter zerologWriter
}

func New(rwMap *pkg.RWMap) *Git {
	return &Git{
		filesMap:       rwMap,
		progressWriter: zerologWriter{log: log.Logger},
	}
}

func (g *Git) Run(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config) (WebhookMap, error) {
	webhooksMap := make(WebhookMap)
	repoNames := make(map[string]struct{})

	log.Info().Msg("Starting initial cloning for Git repos")

	for _, repoConfig := range cfg.Repos {
		_, ok := repoNames[repoConfig.Name]
		if ok {
			log.Warn().Msgf("Repo with name %s (URL %s) duplicated in config, will be ignored", repoConfig.Name, repoConfig.URL)

			continue
		}
		repoNames[repoConfig.Name] = struct{}{}

		repo, err := g.clone(&repoConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to clone repo %s: %w", repoConfig.Name, err)
		}

		if err := g.fetch(repo, true); err != nil {
			return nil, fmt.Errorf("failed initial fetch from repo %s: %w", repo.config.Name, err)
		}

		webhookChan := make(chan struct{})

		if repo.config.Webhook {
			endpoint := "/webhooks/" + repoConfig.Name
			webhooksMap[endpoint] = webhookChan
		}

		// var ticker *time.Ticker
		// if repoConfig.Poll != 0 {
		ticker := time.NewTicker(repoConfig.Poll)
		// }

		wg.Add(1)
		go func() {
			defer wg.Done()

			fetch := func() {
				if err := g.fetch(repo, true); err != nil {
					log.Error().Err(err).Msg("Failed to fetch changes from Git repo")
				}
			}

			for {
				select {
				case <-ticker.C:
					log.Info().Msgf("Starting fetch operation for repo %s on poll", repo.config.Name)
					go fetch()
				case <-webhookChan:
					log.Info().Msgf("Starting fetch operation for repo %s on webhook", repo.config.Name)
					go fetch()
				case <-ctx.Done():
					ticker.Stop()
					close(webhookChan)
					return
				}
			}
		}()
	}

	log.Info().Msg("Initial Git repo cloning complete")

	return webhooksMap, nil
}

func (g *Git) clone(repo *config.Repo) (*gitRepo, error) {
	log.Info().Msgf("Cloning repo %s at URL %s, revision %s", repo.Name, repo.URL, repo.Revision)
	fs := memfs.New()
	storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	auth := &http.BasicAuth{
		Username: repo.Auth.Username,
		Password: repo.Auth.Password,
	}

	cloned, err := git.Clone(storage, nil, &git.CloneOptions{
		URL:      repo.URL,
		Auth:     auth,
		Progress: g.progressWriter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone %s, %w", repo.Name, err)
	}

	zerolog.NewLevelHook()

	gitRepo := &gitRepo{
		config: repo,
		data:   cloned,
		auth:   auth,
	}

	return gitRepo, nil
}

func (g *Git) fetch(repo *gitRepo, initial bool) error {
	err := repo.data.Fetch(&git.FetchOptions{
		Auth:       repo.auth,
		RemoteName: "origin",
		Force:      true,
		Depth:      0,
		RefSpecs: []gitcfg.RefSpec{
			gitcfg.RefSpec("+refs/heads/*:refs/remotes/origin/*"),
		},
		Progress: g.progressWriter,
	})
	if err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return fmt.Errorf("failed to fetch %s: %w", repo.config.Name, err)
		} else if !initial {
			// Repo is already up-to-date, no need to continue with fetch operation
			log.Debug().Msg("Repo already up-to-date")

			return nil
		}
	}

	commit, err := g.resolveCommit(repo.data, repo.config.Revision)
	if err != nil {
		return fmt.Errorf("failed to get commit object for rev %s: %w", repo.config.Revision, err)
	}

	for _, file := range repo.config.Files {
		source := source{
			File: file.Config,
		}

		if file.Template != "" {
			source.File = file.Template
			source.Template = true
		}

		if len(file.Path) == 0 || !strings.HasPrefix(file.Path, "/") {
			log.Warn().Msgf("File %s of repo %s is either empty or does not start with a '/' character, skipping file", source.File, repo.config.Name)

			continue
		}

		log.Debug().Msg("Locating " + source.File)
		f, err := commit.File(source.File)
		if err != nil {
			log.Warn().Err(err).Msgf("File %s not found: ", source.File)
			continue
		}
		log.Debug().Msg("File found: " + f.Name)

		contents, err := f.Contents()
		if err != nil {
			log.Warn().Err(err).Msgf("File %s contents not retreivable", f.Name)
			continue
		}

		if source.Template {
			contents, err = g.compileTemplate(source.File, contents, file.Values)
			if err != nil {
				return err
			}
		}

		g.filesMap.Set("/"+repo.config.Name+file.Path, contents)
	}

	if len(g.filesMap.GetKeys()) == 0 {
		log.Warn().Msgf("No files for repo %s were parsable!", repo.config.Name)
	}

	return nil
}

func (*Git) resolveCommit(repo *git.Repository, revision string) (*object.Commit, error) {
	tryRefs := []plumbing.ReferenceName{
		plumbing.NewBranchReferenceName(revision),           // refs/heads/main
		plumbing.NewRemoteReferenceName("origin", revision), // refs/remotes/origin/main
		plumbing.ReferenceName(revision),                    // fully qualified (in case user passed one)
	}

	for _, refName := range tryRefs {
		ref, err := repo.Reference(refName, true)
		if err == nil {
			return repo.CommitObject(ref.Hash())
		}
	}

	// Try to treat the revision as a direct hash or tag
	hash, err := repo.ResolveRevision(plumbing.Revision(revision))
	if err == nil {
		return repo.CommitObject(*hash)
	}

	return nil, fmt.Errorf("could not resolve commit for revision %q: %w", revision, err)
}

func (*Git) compileTemplate(filename, contents string, values []config.Value) (string, error) {
	tmpl, err := template.New(filename).Parse(contents)
	if err != nil {
		return "", fmt.Errorf("could not parse template %s, %w", filename, err)
	}

	valMap := make(map[string]any)
	for _, val := range values {
		valMap[val.Key] = val.Value
	}

	var result bytes.Buffer
	if err = tmpl.Execute(&result, valMap); err != nil {
		return "", fmt.Errorf("could not execute template %s: %w", filename, err)
	}

	return result.String(), nil
}
