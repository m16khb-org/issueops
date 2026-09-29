package apidoc

import app "issueops/internal/application/apidoc"

type Command struct {
	Service       app.Service
	ResolveTarget func(string) string
}

func (c Command) Run(args []string) error { return c.runAPIDoc(args) }
