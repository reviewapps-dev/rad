package initializer

import (
	_ "embed"
	"os"
)

//go:embed _reviewapps.rb.tmpl
var template string

func Write(path string) error {
	return os.WriteFile(path, []byte(template), 0644)
}
