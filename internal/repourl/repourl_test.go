package repourl

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  Repo
	}{
		{"https://github.com/user/project.git", Repo{"github.com", "user", "project", "https://github.com/user/project.git"}},
		{"https://gitlab.com/user/project/", Repo{"gitlab.com", "user", "project", "https://gitlab.com/user/project/"}},
		{"ssh://git@github.com:22/user/project.git", Repo{"github.com", "user", "project", "ssh://git@github.com:22/user/project.git"}},
		{"git@github.com:user/project.git", Repo{"github.com", "user", "project", "git@github.com:user/project.git"}},
		{"example.com:user/project", Repo{"example.com", "user", "project", "example.com:user/project"}},
		{"gitlab.com/user/project", Repo{"gitlab.com", "user", "project", "https://gitlab.com/user/project"}},
		{"user/project", Repo{"example.org", "user", "project", "https://example.org/user/project"}},
		{"user/project.git", Repo{"example.org", "user", "project", "https://example.org/user/project.git"}},
		{"/tmp/source-repo", Repo{"localhost", "local", "source-repo", "/tmp/source-repo"}},
		{"/tmp/a/b/", Repo{"localhost", "local", "b", "/tmp/a/b/"}},
		{"./foo/bar.git", Repo{"localhost", "local", "bar", "./foo/bar.git"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input, "example.org")
			if err != nil || got != tt.want {
				t.Errorf("got (%+v, %v), want %+v", got, err, tt.want)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	for _, input := range []string{"", "project", "a/b/c/d", "https://github.com/user", "git@github.com:project", "/", "https:///user/project"} {
		t.Run(input, func(t *testing.T) {
			if got, err := Parse(input, "github.com"); err == nil {
				t.Errorf("expected an error, got %+v", got)
			}
		})
	}
}
