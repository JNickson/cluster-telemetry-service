package nodes

import (
	"testing"

	"github.com/JNickson/cluster-telemetry-service/internal/testutil"
	v1 "k8s.io/api/core/v1"
)

type mapNodeInput struct {
	Node      v1.Node
	Usage     v1.ResourceList
	Workloads NodeWorkloads
}

func TestMapNode(t *testing.T) {
	testutil.RunGoldenTest(
		t,
		"testdata/mapNode",
		func(input mapNodeInput) Node {
			return mapNode(&input.Node, input.Usage, input.Workloads)
		},
	)
}
