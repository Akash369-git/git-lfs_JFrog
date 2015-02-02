package hawser

import (
	"crypto/tls"
	"fmt"
	"github.com/hawser/git-hawser/git"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

type Configuration struct {
	gitConfig  map[string]string
	remotes    []string
	httpClient *http.Client
}

var (
	Config        = &Configuration{}
	httpPrefixRe  = regexp.MustCompile("\\Ahttps?://")
	RedirectError = fmt.Errorf("Unexpected redirection")
)

func HttpClient() *http.Client {
	return Config.HttpClient()
}

func (c *Configuration) HttpClient() *http.Client {
	if c.httpClient == nil {
		tr := &http.Transport{}
		sslVerify, _ := c.GitConfig("http.sslverify")
		if len(os.Getenv("GIT_SSL_NO_VERIFY")) > 0 || sslVerify == "false" {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		c.httpClient = &http.Client{Transport: tr}
		c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return RedirectError
		}
	}
	return c.httpClient
}

func (c *Configuration) Endpoint() string {
	if url, ok := c.GitConfig("hawser.url"); ok {
		return url
	}

	return c.RemoteEndpoint("origin")
}

func (c *Configuration) RemoteEndpoint(remote string) string {
	if url, ok := c.GitConfig("remote." + remote + ".hawser"); ok {
		return url
	}

	if url, ok := c.GitConfig("remote." + remote + ".url"); ok {
		if !httpPrefixRe.MatchString(url) {
			pieces := strings.SplitN(url, ":", 2)
			hostPieces := strings.SplitN(pieces[0], "@", 2)
			if len(hostPieces) < 2 {
				return "unknown"
			}
			url = fmt.Sprintf("https://%s/%s", hostPieces[1], pieces[1])
		}

		if path.Ext(url) == ".git" {
			return url + "/info/media"
		}
		return url + ".git/info/media"
	}

	return ""
}

func (c *Configuration) Remotes() []string {
	if c.remotes == nil {
		c.loadGitConfig()
	}
	return c.remotes
}

func (c *Configuration) GitConfig(key string) (string, bool) {
	if c.gitConfig == nil {
		c.loadGitConfig()
	}
	value, ok := c.gitConfig[strings.ToLower(key)]
	return value, ok
}

func (c *Configuration) SetConfig(key, value string) {
	if c.gitConfig == nil {
		c.loadGitConfig()
	}
	c.gitConfig[key] = value
}

type AltConfig struct {
	Remote map[string]*struct {
		Media string
	}

	Media struct {
		Url string
	}
}

func (c *Configuration) loadGitConfig() {
	uniqRemotes := make(map[string]bool)

	c.gitConfig = make(map[string]string)

	var output string
	listOutput, err := git.Config.List()
	if err != nil {
		panic(fmt.Errorf("Error listing git config: %s", err))
	}

	fileOutput, err := git.Config.ListFromFile()
	if err != nil {
		panic(fmt.Errorf("Error listing git config from file: %s", err))
	}

	output = listOutput + "\n" + fileOutput

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		pieces := strings.SplitN(line, "=", 2)
		if len(pieces) < 2 {
			continue
		}
		key := strings.ToLower(pieces[0])
		c.gitConfig[key] = pieces[1]

		keyParts := strings.Split(key, ".")
		if len(keyParts) > 1 && keyParts[0] == "remote" {
			remote := keyParts[1]
			uniqRemotes[remote] = remote == "origin"
		}
	}

	c.remotes = make([]string, 0, len(uniqRemotes))
	for remote, isOrigin := range uniqRemotes {
		if isOrigin {
			continue
		}
		c.remotes = append(c.remotes, remote)
	}
}

func configFileExists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}