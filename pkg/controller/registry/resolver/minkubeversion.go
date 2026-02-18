package resolver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"

	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry/resolver/cache"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry/resolver/solver"
	"github.com/operator-framework/operator-registry/alpha/property"
)

// NewMinKubeVersionConstraintProvider returns a ConstraintProviderFunc that prohibits
// bundles whose minKubeVersion exceeds the given server version. Bundles without a
// minKubeVersion property, or with an unparseable one, are allowed through (fail-open).
func NewMinKubeVersionConstraintProvider(serverVersion string, log logrus.FieldLogger) (ConstraintProviderFunc, error) {
	sv, err := semver.Parse(serverVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid server version %q: %w", serverVersion, err)
	}

	return func(e *cache.Entry) ([]solver.Constraint, error) {
		minKube := extractMinKubeVersionFromEntry(e, log)
		if minKube == "" {
			return nil, nil
		}

		mkv, err := semver.Parse(strings.TrimPrefix(minKube, "v"))
		if err != nil {
			log.Warnf("bundle %s has invalid minKubeVersion %q, skipping constraint: %v", e.Name, minKube, err)
			return nil, nil
		}

		if mkv.GT(sv) {
			return []solver.Constraint{PrettyConstraint(
				solver.Prohibited(),
				fmt.Sprintf("bundle %s requires minKubeVersion %s which is incompatible with cluster version %s",
					e.Name, minKube, serverVersion),
			)}, nil
		}

		return nil, nil
	}, nil
}

// extractMinKubeVersionFromEntry extracts minKubeVersion from a cache.Entry's properties.
// Returns empty string if the property is not present or cannot be parsed.
func extractMinKubeVersionFromEntry(e *cache.Entry, log logrus.FieldLogger) string {
	for _, p := range e.Properties {
		if p.GetType() != property.TypeCSVMetadata {
			continue
		}

		// Use a minimal struct to avoid allocating the full property.CSVMetadata
		var meta struct {
			MinKubeVersion string `json:"minKubeVersion"`
		}
		if err := json.Unmarshal([]byte(p.GetValue()), &meta); err != nil {
			log.Warnf("bundle %s has malformed %s property, skipping: %v", e.Name, property.TypeCSVMetadata, err)
			continue
		}

		if meta.MinKubeVersion != "" {
			return meta.MinKubeVersion
		}
	}
	return ""
}
