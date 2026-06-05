package export

import (
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/view"
)

func testLogger() log.Logger {
	return log.Default()
}

func testDependenciesModel() view.DependenciesModel {
	return view.DependenciesModel{
		Entities:   []view.IdModel{},
		Groups:     []view.GroupModel{},
		References: []view.RefModel{},
	}
}
