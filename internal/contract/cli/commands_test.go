package cli

import (
	"strings"
	"testing"
)

func TestCommandCatalogsHaveUniqueKebabNamesAndDescriptions(t *testing.T) {
	t.Parallel()
	seen := map[string]string{}
	for catalog, commands := range map[string][]Command{"root": RootCommands(), "lifecycle": LifecycleCommands()} {
		if len(commands) == 0 {
			t.Fatalf("%s catalog is empty", catalog)
		}
		for _, command := range commands {
			if command.Name == "" || command.Name != strings.TrimSpace(command.Name) || command.Name != strings.ToLower(command.Name) || strings.ContainsAny(command.Name, " _") {
				t.Errorf("%s command name %q must be a non-empty lower-case kebab token", catalog, command.Name)
			}
			if strings.TrimSpace(command.Description) == "" {
				t.Errorf("%s command %q has an empty description", catalog, command.Name)
			}
			if previous, dup := seen[command.Name]; dup {
				t.Errorf("command %q appears in both %s and %s catalogs or twice in one", command.Name, previous, catalog)
			}
			seen[command.Name] = catalog
		}
	}
}

func TestLifecycleCommandCatalogIsAlphabetical(t *testing.T) {
	t.Parallel()
	commands := LifecycleCommands()
	for i := 1; i < len(commands); i++ {
		if commands[i-1].Name >= commands[i].Name {
			t.Errorf("lifecycle commands out of order: %q before %q", commands[i-1].Name, commands[i].Name)
		}
	}
}
