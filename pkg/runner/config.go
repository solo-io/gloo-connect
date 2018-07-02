package runner

import (
	"github.com/solo-io/gloo/pkg/bootstrap"
)

type RunConfig struct {
	Options          bootstrap.Options
	GlooAddress      string
	GlooPort         uint
	ConfigDir        string
	EnvoyPath        string
}
