package pods

import (
	"testing"

	"github.com/JNickson/cluster-telemetry-service/internal/testutil"
	v1 "k8s.io/api/core/v1"
)

// Golden File Tests

func TestMapPod(t *testing.T) {
	testutil.RunGoldenTest(
		t,
		"testdata/mapPod",
		func(input v1.Pod) Pod {
			return mapPod(input)
		},
	)
}
