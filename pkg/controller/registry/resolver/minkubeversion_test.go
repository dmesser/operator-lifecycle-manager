package resolver

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry/resolver/cache"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry/resolver/solver"
	"github.com/operator-framework/operator-registry/pkg/api"
)

func TestNewMinKubeVersionConstraintProvider(t *testing.T) {
	tests := []struct {
		name           string
		serverVersion  string
		expectError    bool
	}{
		{
			name:          "valid server version",
			serverVersion: "1.28.0",
			expectError:   false,
		},
		{
			name:          "invalid server version",
			serverVersion: "invalid",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMinKubeVersionConstraintProvider(tt.serverVersion, logrus.New())
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMinKubeVersionConstraints(t *testing.T) {
	tests := []struct {
		name            string
		serverVersion   string
		entry           *cache.Entry
		expectProhibit  bool
	}{
		{
			name:          "no properties - no constraint",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name:       "test-operator.v1",
				Properties: nil,
			},
			expectProhibit: false,
		},
		{
			name:          "no csv.metadata property - no constraint",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.package", Value: `{"packageName":"test","version":"1.0.0"}`},
				},
			},
			expectProhibit: false,
		},
		{
			name:          "csv.metadata without minKubeVersion - no constraint",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"displayName":"Test Operator"}`},
				},
			},
			expectProhibit: false,
		},
		{
			name:          "minKubeVersion less than server - no constraint",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"1.24.0"}`},
				},
			},
			expectProhibit: false,
		},
		{
			name:          "minKubeVersion equal to server - no constraint",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"1.28.0"}`},
				},
			},
			expectProhibit: false,
		},
		{
			name:          "minKubeVersion greater than server - prohibited",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"1.30.0"}`},
				},
			},
			expectProhibit: true,
		},
		{
			name:          "minKubeVersion with v prefix less than server - no constraint",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"v1.24.0"}`},
				},
			},
			expectProhibit: false,
		},
		{
			name:          "minKubeVersion with v prefix greater than server - prohibited",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"v1.30.0"}`},
				},
			},
			expectProhibit: true,
		},
		{
			name:          "invalid minKubeVersion format - no constraint (fail-open)",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"invalid"}`},
				},
			},
			expectProhibit: false,
		},
		{
			name:          "invalid csv.metadata JSON - no constraint (fail-open)",
			serverVersion: "1.28.0",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `not valid json`},
				},
			},
			expectProhibit: false,
		},
		{
			name:          "patch version comparison - greater patch prohibited",
			serverVersion: "1.28.3",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"1.28.5"}`},
				},
			},
			expectProhibit: true,
		},
		{
			name:          "minor version comparison - greater minor prohibited",
			serverVersion: "1.28.10",
			entry: &cache.Entry{
				Name: "test-operator.v1",
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"1.29.0"}`},
				},
			},
			expectProhibit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewMinKubeVersionConstraintProvider(tt.serverVersion, logrus.New())
			require.NoError(t, err)

			constraints, err := provider.Constraints(tt.entry)
			require.NoError(t, err)

			if tt.expectProhibit {
				require.Len(t, constraints, 1, "expected one Prohibited constraint")
				// Verify it's a Prohibited constraint by checking its String output
				// prettyConstraint.String takes a solver.Identifier parameter
				type stringer interface {
					String(solver.Identifier) string
				}
				msg := constraints[0].(stringer).String(solver.Identifier(""))
				require.Contains(t, msg, "minKubeVersion")
				require.Contains(t, msg, tt.entry.Name)
			} else {
				require.Empty(t, constraints, "expected no constraints")
			}
		})
	}
}

func TestExtractMinKubeVersionFromEntry(t *testing.T) {
	tests := []struct {
		name     string
		entry    *cache.Entry
		expected string
	}{
		{
			name: "entry with minKubeVersion",
			entry: &cache.Entry{
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"minKubeVersion":"1.25.0"}`},
				},
			},
			expected: "1.25.0",
		},
		{
			name: "entry without minKubeVersion",
			entry: &cache.Entry{
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `{"displayName":"Test"}`},
				},
			},
			expected: "",
		},
		{
			name: "entry with no properties",
			entry: &cache.Entry{
				Properties: nil,
			},
			expected: "",
		},
		{
			name: "entry with non-csv.metadata properties only",
			entry: &cache.Entry{
				Properties: []*api.Property{
					{Type: "olm.package", Value: `{"packageName":"test","version":"1.0.0"}`},
				},
			},
			expected: "",
		},
		{
			name: "entry with invalid JSON in csv.metadata",
			entry: &cache.Entry{
				Properties: []*api.Property{
					{Type: "olm.csv.metadata", Value: `not json`},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMinKubeVersionFromEntry(tt.entry, logrus.New())
			require.Equal(t, tt.expected, result)
		})
	}
}
