package templates

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed *.tmpl
var fs embed.FS

type Data struct {
	FlagsContext string
}

func Render(name string, data Data) (string, error) {
	raw, err := fs.ReadFile(name)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
