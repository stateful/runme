package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

const usage = `Usage: releasenotes -base BASE -head HEAD

This is a simple tool that returns a list of commits between two tags.

The output is in Markdown format so that it's easy to publish this
in GitHub Release notes, etc.

Authentication is based on a token. It needs to be available
as an environment variable named GITHUB_TOKEN.
`

var tpl = template.Must(template.New("").Funcs(template.FuncMap{"split": split}).Parse(`## Download

### macOS:

* [runme_darwin_x86_64.tar.gz](https://download.stateful.com/runme/{{ .Version }}/runme_darwin_x86_64.tar.gz)
* [runme_darwin_arm64.tar.gz](https://download.stateful.com/runme/{{ .Version }}/runme_darwin_arm64.tar.gz)

### Linux

* [runme_linux_x86_64.deb](https://download.stateful.com/runme/{{ .Version }}/runme_linux_x86_64.deb)
* [runme_linux_arm64.deb](https://download.stateful.com/runme/{{ .Version }}/runme_linux_arm64.deb)
* [runme_linux_x86_64.rpm](https://download.stateful.com/runme/{{ .Version }}/runme_linux_x86_64.rpm)
* [runme_linux_arm64.rpm](https://download.stateful.com/runme/{{ .Version }}/runme_linux_arm64.rpm)
* [runme_linux_x86_64.apk](https://download.stateful.com/runme/{{ .Version }}/runme_linux_x86_64.apk)
* [runme_linux_arm64.apk](https://download.stateful.com/runme/{{ .Version }}/runme_linux_arm64.apk)
* [runme_linux_x86_64.tar.gz](https://download.stateful.com/runme/{{ .Version }}/runme_linux_x86_64.tar.gz)
* [runme_linux_arm64.tar.tz](https://download.stateful.com/runme/{{ .Version }}/runme_linux_arm64.tar.gz)

### Windows

* [runme_windows_x86_64.zip](https://download.stateful.com/runme/{{ .Version }}/runme_windows_x86_64.zip)
* [runme_windows_arm64.zip](https://download.stateful.com/runme/{{ .Version }}/runme_windows_arm64.zip)

## Changelog

[Full changelog](https://github.com/stateful/runme/compare/v{{ or .PreviousVersion "main" }}...v{{ .Version }})

{{ range $value := .Commits -}}
* {{ $value.SHA }}: {{ (split "\n" $value.Commit.Message)._0 }}
{{- if $value.Author }} ([@{{ $value.Author.Login }}]({{ $value.Author.HTMLURL }}))
{{- else if $value.Commit.Author }} ({{ $value.Commit.Author.Name }}){{ end }}
{{ end }}
`))

type tplData struct {
	PreviousVersion string // without 'v' prefix
	Version         string // without 'v' prefix
	Commits         []*github.RepositoryCommit
}

type config struct {
	Owner   string
	Repo    string
	Version string
}

func (c config) IsPreRelease() bool {
	re := regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	return re.Match([]byte(strings.TrimLeft(c.Version, "v")))
}

var flagsConfig = config{}

func init() {
	flag.StringVar(&flagsConfig.Owner, "owner", "stateful", "Owner of the repository")
	flag.StringVar(&flagsConfig.Repo, "repo", "runme", "Repository name")
	flag.StringVar(&flagsConfig.Version, "version", "", "The new version, for example v0.1.1")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", usage)
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if flagsConfig.Version == "" {
		log.Fatalf("-version cannot be empty")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-sigs
		cancel()
	}()

	client := ghClient(ctx)

	var (
		releases     []*github.RepositoryRelease
		releasesPage int
	)

	for {
		result, resp, err := client.Repositories.ListReleases(ctx, flagsConfig.Owner, flagsConfig.Repo, &github.ListOptions{
			Page:    releasesPage,
			PerPage: 100,
		})
		if err != nil {
			log.Fatal(err)
		}

		releases = append(releases, result...)

		if resp.NextPage == 0 {
			break
		}

		releasesPage = resp.NextPage
	}

	sort.Slice(releases, func(i, j int) bool {
		return releases[i].GetCreatedAt().After(releases[j].GetCreatedAt().Time)
	})

	var (
		currentTag       = flagsConfig.Version
		currentTagParsed = semver.MustParse(currentTag)
		previousRelease  *github.RepositoryRelease
	)

	for _, r := range releases {
		if r.GetTagName() == currentTag {
			continue
		}

		tagParsed, err := semver.NewVersion(r.GetTagName())
		if err != nil {
			log.Fatal(err)
		}

		if currentTagParsed.LessThan(tagParsed) {
			continue
		}

		if !isPrerelease(currentTag) {
			if !r.GetPrerelease() {
				previousRelease = r
			}
		} else {
			previousRelease = r
		}

		if previousRelease != nil {
			break
		}
	}

	var (
		commits                []*github.RepositoryCommit
		commitsPage            int
		previousReleaseTagName string
	)

	if previousRelease != nil {
		previousReleaseTagName = previousRelease.GetTagName()
	} else {
		previousReleaseTagName = "main"
	}

	for {
		result, resp, err := client.Repositories.CompareCommits(ctx, flagsConfig.Owner, flagsConfig.Repo, previousReleaseTagName, currentTag, &github.ListOptions{
			Page:    commitsPage,
			PerPage: 100,
		})
		if err != nil {
			log.Fatal(err)
		}

		commits = append(commits, result.Commits...)

		if resp.NextPage == 0 {
			break
		}

		commitsPage = resp.NextPage
	}

	data := tplData{
		PreviousVersion: strings.TrimLeft(previousReleaseTagName, "v"),
		Version:         strings.TrimLeft(currentTag, "v"),
		Commits:         commits,
	}

	if err := tpl.Execute(os.Stdout, data); err != nil {
		log.Fatal(err)
	}
}

func ghClient(ctx context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func split(sep, orig string) map[string]string {
	parts := strings.Split(orig, sep)
	res := make(map[string]string, len(parts))
	for i, v := range parts {
		res["_"+strconv.Itoa(i)] = v
	}
	return res
}

func isPrerelease(tag string) bool {
	re := regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	return re.MatchString(strings.TrimLeft(tag, "v"))
}
