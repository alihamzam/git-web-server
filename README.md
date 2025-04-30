# Git Web Server

Serves files from a Git repo, as mapped by a config file.

Can server a specific file at a specific path, and can template files with given key-value pairs.

Can poll a Git repo to look for changes. If a change is detected, pulls the latest revision and serves the new files.

## Example use

See `config.yaml` for a sample config file. If a rwebhook for a repo is enabled, it'll be available at URL /webhooks/{repo_name}.
