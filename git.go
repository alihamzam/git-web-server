package main

import (
	"bytes"
	"os"
	"text/template"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/rs/zerolog/log"
)

type Source struct {
	File     string
	Template bool
}

func Obtain(c *Config) (map[string]string, error) {
	endpoints := make(map[string]string)

	for _, repo := range c.Repos {
		cloned, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL: repo.URL,
			Auth: &http.BasicAuth{
				Username: repo.Auth.Username,
				Password: repo.Auth.Password,
			},
			Progress: os.Stdout,
		})
		if err != nil {
			return nil, err
		}

		h, err := cloned.ResolveRevision(plumbing.Revision(repo.Revision))
		if err != nil {
			return nil, err
		}
		log.Info().Msg("rev parsed: " + h.String())

		commit, err := cloned.CommitObject(*h)
		if err != nil {
			return nil, err
		}

		for _, file := range repo.Files {
			source := Source{
				File: file.Config,
			}

			if file.Template != "" {
				source.File = file.Template
				source.Template = true
			}

			log.Info().Msg("Locating " + source.File)
			f, err := commit.File(source.File)
			if err != nil {
				log.Warn().Err(err).Msg("File not found: " + source.File)
				continue
			}
			log.Info().Msg("File found: " + f.Name)

			contents, err := f.Contents()
			if err != nil {
				log.Warn().Err(err).Msg("File contents not retreivable")
			}

			if source.Template {
				contents, err = CompileTemplate(source.File, contents, file.Values)
				if err != nil {
					return nil, err
				}
			}

			endpoints[file.Path] = contents
		}
	}

	return endpoints, nil
}

func CompileTemplate(filename, contents string, values []Value) (string, error) {
	tmpl, err := template.New(filename).Parse(contents)
	if err != nil {
		return "", err
	}

	valMap := make(map[string]any)
	for _, val := range values {
		valMap[val.Key] = val.Value
	}

	var result bytes.Buffer
	if err = tmpl.Execute(&result, valMap); err != nil {
		return "", err
	}

	return result.String(), nil
}
