package helm

import "testing"

func TestHelmRepoUpdateGate(t *testing.T) {
	t.Parallel()
	var g helmRepoUpdateGate

	if g.shouldSkipRepoUpdate() {
		t.Fatal("first dependency download should run helm repo update (SkipUpdate=false)")
	}
	g.markRepoUpdated()
	if !g.shouldSkipRepoUpdate() {
		t.Fatal("after a successful download, further charts should skip repo update (SkipUpdate=true)")
	}
}
