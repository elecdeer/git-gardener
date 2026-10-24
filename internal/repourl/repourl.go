// Package repourl はリポジトリ URL や省略形を host/owner/name に分解する。
package repourl

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// Repo は分解されたリポジトリの所在。
type Repo struct {
	Host  string
	Owner string
	Name  string
	// URL は git clone に渡す URL。
	URL string
}

// splitOwnerName は "owner/name(.git)" 形式のパスを分解する。
func splitOwnerName(path string) (owner, name string, ok bool) {
	parts := strings.Split(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// Parse は input を解釈する。受け付ける形式は以下。
//   - https://host/owner/name(.git), http://..., ssh://[user@]host[:port]/owner/name(.git)
//   - [user@]host:owner/name(.git)（scp 形式）
//   - host/owner/name, owner/name（ホスト省略時は defaultHost、https でクローンする）
//   - /abs/path, ./rel/path（ローカルパス。localhost/local/<ディレクトリ名> に配置する）
func Parse(input, defaultHost string) (Repo, error) {
	invalid := fmt.Errorf("invalid repository URL format: %s", input)

	if strings.HasPrefix(input, "/") || strings.HasPrefix(input, ".") {
		name := filepath.Base(filepath.Clean(input))
		if name == "/" || name == "." || name == ".." {
			return Repo{}, invalid
		}
		return Repo{Host: "localhost", Owner: "local", Name: strings.TrimSuffix(name, ".git"), URL: input}, nil
	}

	if strings.Contains(input, "://") {
		u, err := url.Parse(input)
		if err != nil || u.Hostname() == "" {
			return Repo{}, invalid
		}
		owner, name, ok := splitOwnerName(u.Path)
		if !ok {
			return Repo{}, invalid
		}
		return Repo{Host: u.Hostname(), Owner: owner, Name: name, URL: input}, nil
	}

	// scp 形式: [user@]host:owner/name
	if hostPart, path, found := strings.Cut(input, ":"); found {
		host := hostPart
		if _, after, ok := strings.Cut(hostPart, "@"); ok {
			host = after
		}
		owner, name, ok := splitOwnerName(path)
		if host == "" || !ok {
			return Repo{}, invalid
		}
		return Repo{Host: host, Owner: owner, Name: name, URL: input}, nil
	}

	parts := strings.Split(strings.TrimSuffix(input, ".git"), "/")
	switch {
	case len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != "":
		return Repo{Host: parts[0], Owner: parts[1], Name: parts[2], URL: "https://" + input}, nil
	case len(parts) == 2 && parts[0] != "" && parts[1] != "":
		return Repo{Host: defaultHost, Owner: parts[0], Name: parts[1], URL: "https://" + defaultHost + "/" + input}, nil
	}
	return Repo{}, invalid
}
