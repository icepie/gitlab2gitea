# gitlab2gitea [![Go Report Card](https://goreportcard.com/badge/icepie/gitlab2gitea)](https://goreportcard.com/report/github.com/icepie/gitlab2gitea)

A command line tool build with Golang to migrate a [GitLab](https://gitlab.com/) project to [Gitea](https://gitea.io/).

It uses the exposed API of both systems to migrate following data of a project:

* All repo
* All milestones
* All labels
* All issues

It skips creation if an item already exists.

## Installation

```
go install github.com/icepie/gitlab2gitea@latest
```

Installing the tool from source code needs to have a recent version of [Golang](https://go.dev/) installed.

## Usage

```
./gitlab2gitea --gitlabserver https://gitlab.domain.tld/ --gitlabtoken 12345 \
--giteaserver https://gitea.domain.tld/ --giteatoken 54321
```

## Options

```
Migrate repo, labels, issues and milestones from GitLab to Gitea.

Usage: gitlab2gitea --gitlabtoken GITLABTOKEN --gitlabproject GITLABPROJECT --giteatoken GITEATOKEN --giteaserver GITEASERVER

Options:
  --gitlabtoken GITLABTOKEN
                         token for GitLab API access
  --gitlabserver GITLABSERVER
                         GitLab server URL with a trailing slash
  --giteatoken GITEATOKEN
                         token for Gitea API access
  --giteaserver GITEASERVER
                         Gitea server URL
  --help, -h             display this help and exit
```
