package main

import (
	"fmt"
	"os"
)

func setup(home string) error {
	return os.MkdirAll(fmt.Sprintf("%s/.local/share/evergreen", home), 0o755)
}
