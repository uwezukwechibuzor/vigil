package cmd_test

import (
	"testing"

	"github.com/uwezukwechibuzor/vigil/cli/internal/cmd"
)

func TestRootHelp(t *testing.T) {
	root := cmd.Root()
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRootHasCommands(t *testing.T) {
	root := cmd.Root()
	commands := root.Commands()
	wantNames := map[string]bool{
		"login":     false,
		"check":     false,
		"health":    false,
		"runs":      false,
		"incidents": false,
	}
	for _, c := range commands {
		if _, ok := wantNames[c.Name()]; ok {
			wantNames[c.Name()] = true
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Errorf("missing command: %s", name)
		}
	}
}
