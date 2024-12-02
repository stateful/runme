package runnerv2service

import (
	"testing/fstest"

	"github.com/stateful/runme/v3/internal/command"
	"github.com/stateful/runme/v3/internal/config"
	"github.com/stateful/runme/v3/internal/config/autoconfig"
)

func init() {
	command.SetEnvDumpCommandForTesting()

	// Server uses autoconfig to get necessary dependencies.
	// One of them, implicit, is [config.Config]. With the default
	// [config.Loader] it won't be found during testing, so
	// we need to provide an override.
	if err := autoconfig.DecorateRoot(func(loader *config.Loader) *config.Loader {
		fsys := fstest.MapFS{
			"runme.yaml": {
				Data: []byte("version: v1alpha1\n"),
			},
		}
		loader.SetConfigRootPath(fsys)
		return loader
	}); err != nil {
		panic(err)
	}
}
