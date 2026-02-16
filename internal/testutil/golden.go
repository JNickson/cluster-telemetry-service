package testutil

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JNickson/cluster-telemetry-service/internal/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

var Update = flag.Bool("update", false, "update .golden files")

func RunGoldenTest[In any, Out any](
	t *testing.T,
	dir string,
	exec func(input In) Out,
) {

	inputFiles, err := filepath.Glob(filepath.Join(dir, "*.input.json"))
	require.NoError(t, err)

	if len(inputFiles) == 0 {
		t.Fatalf("no input files found in %s", dir)
	}

	for _, inputPath := range inputFiles {
		name := filepath.Base(inputPath)

		t.Run(name, func(t *testing.T) {

			originalNow := utils.Now
			utils.Now = func() time.Time {
				return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			}
			defer func() { utils.Now = originalNow }()

			var input In
			readJSON(t, inputPath, &input)

			result := exec(input)

			goldenPath := strings.Replace(inputPath, ".input.json", ".golden.json", 1)

			if *Update && os.Getenv("CI") == "true" {
				t.Fatal("golden file updates are not allowed in CI")
			}

			if *Update {
				writeGolden(t, goldenPath, result)
				return
			}

			var expected Out
			readJSON(t, goldenPath, &expected)

			if diff := cmp.Diff(expected, result); diff != "" {
				t.Fatalf(
					"mismatch (-expected +actual):\n%s\n\n"+
						"If this change is intentional, run:\n"+
						" go test ./... -args -update\n",
					diff,
				)
			}
		})
	}
}

func readJSON(t *testing.T, path string, v any) {
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, v))
}

func writeGolden[T any](t *testing.T, path string, result T) {
	t.Helper()

	var old T

	if existing, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(existing, &old)

		if diff := cmp.Diff(old, result); diff != "" {
			t.Logf("Updating golden %s:\n%s", path, diff)
		}
	} else {
		t.Logf("Creating golden %s", path)
	}

	out, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, out, 0644))
}
