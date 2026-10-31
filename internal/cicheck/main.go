// cicheck is a CI-only confinement assertion and release-record writer. It is
// never included in the application image.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func run(args []string) error {
	if len(args) == 1 && args[0] == "preflight" {
		if err := checkConfinement(os.DirFS("/"), os.Getuid(), os.Getgid(), os.Environ()); err != nil {
			return err
		}
		fmt.Println("confinement: uid/gid=1001, capabilities=none, no-new-privileges, seccomp=filter, no cluster token/runtime socket/ambient credentials")
		return nil
	}
	if len(args) == 4 && args[0] == "release" {
		refs, err := os.ReadFile(args[3])
		if err != nil {
			return fmt.Errorf("read published image reference: %w", err)
		}
		value, err := newRelease(args[1], args[2], string(refs))
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	return fmt.Errorf("usage: cicheck preflight | cicheck release SOURCE_SHA RUN_ID IMAGE_REFS_FILE")
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
