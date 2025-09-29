package apply

import (
    "context"
    "time"

    helmctlv2 "github.com/fluxcd/helm-controller/api/v2"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"

    kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
    "github.com/fidelity/kraan/pkg/layers"
)

// FakeApplier is a simple test double for LayerApplier used in controller tests.
type FakeApplier struct {
    Hrs map[string]*helmctlv2.HelmRelease
}

func (f FakeApplier) Apply(ctx context.Context, layer layers.Layer) error { return nil }
func (f FakeApplier) Prune(ctx context.Context, layer layers.Layer, pruneHrs []*helmctlv2.HelmRelease) error {
    return nil
}
func (f FakeApplier) PruneIsRequired(ctx context.Context, layer layers.Layer) (bool, []*helmctlv2.HelmRelease, error) {
    return false, nil, nil
}
func (f FakeApplier) ApplyIsRequired(ctx context.Context, layer layers.Layer) (bool, error) { return false, nil }
func (f FakeApplier) ApplyWasSuccessful(ctx context.Context, layer layers.Layer) (bool, string, error) {
    return false, "", nil
}
func (f FakeApplier) GetResources(ctx context.Context, layer layers.Layer) ([]kraanv1alpha1.Resource, error) {
    return nil, nil
}
func (f FakeApplier) GetSourceAndClusterHelmReleases(ctx context.Context, layer layers.Layer) (map[string]*helmctlv2.HelmRelease, map[string]*helmctlv2.HelmRelease, error) {
    return map[string]*helmctlv2.HelmRelease{}, map[string]*helmctlv2.HelmRelease{}, nil
}
func (f FakeApplier) Orphan(ctx context.Context, layer layers.Layer, hr *helmctlv2.HelmRelease) (bool, error) {
    return false, nil
}
func (f FakeApplier) GetOrphanedHelmReleases(ctx context.Context, layer layers.Layer) (map[string]*helmctlv2.HelmRelease, error) {
    return map[string]*helmctlv2.HelmRelease{}, nil
}
func (f FakeApplier) Adopt(ctx context.Context, layer layers.Layer, hr *helmctlv2.HelmRelease) error { return nil }
func (f FakeApplier) addOwnerRefs(layer layers.Layer, objs []runtime.Object) error { return nil }
func (f FakeApplier) orphanLabel(ctx context.Context, hr *helmctlv2.HelmRelease) (*metav1.Time, error) {
    t := metav1.NewTime(time.Now())
    return &t, nil
}
func (f FakeApplier) GetHelmReleases(ctx context.Context, layer layers.Layer) (map[string]*helmctlv2.HelmRelease, error) {
    // Return a copy of the map to avoid accidental aliasing
    out := map[string]*helmctlv2.HelmRelease{}
    for k, v := range f.Hrs {
        out[k] = v
    }
    return out, nil
}
