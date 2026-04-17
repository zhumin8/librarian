package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/googleapis/librarian/internal/config"
	"github.com/googleapis/librarian/internal/serviceconfig"
	"github.com/googleapis/librarian/internal/yaml"
)

func main() {
	// 1. Load all Java library configurations to get their versions.
	javaLibData, err := os.ReadFile("../google-cloud-java/librarian.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read librarian.yaml: %v\n", err)
		os.Exit(1)
	}
	javaCfgPtr, err := yaml.Unmarshal[config.Config](javaLibData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal librarian.yaml: %v\n", err)
		os.Exit(1)
	}
	javaCfg := *javaCfgPtr

	// Map API path to the library version it belongs to.
	apiToVersion := make(map[string]string)
	for _, lib := range javaCfg.Libraries {
		version := lib.Version
		for _, api := range lib.APIs {
			apiToVersion[api.Path] = version
		}
	}

	// 2. Load sdk.yaml
	sdkData, err := os.ReadFile("internal/serviceconfig/sdk.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read sdk.yaml: %v\n", err)
		os.Exit(1)
	}
	apisPtr, err := yaml.Unmarshal[[]serviceconfig.API](sdkData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal sdk.yaml: %v\n", err)
		os.Exit(1)
	}
	apis := *apisPtr

	apiMap := make(map[string]*serviceconfig.API)
	for i := range apis {
		apiMap[apis[i].Path] = &apis[i]
	}

	addedCount := 0
	newPathsCount := 0

	// 3. Process each Java API path
	for path, version := range apiToVersion {
		api, ok := apiMap[path]
		if !ok {
			// Missing from sdk.yaml. Check if heuristic would derive preview.
			// We simulate a temporary API struct to check ReleaseLevel.
			tempAPI := serviceconfig.API{Path: path}
			if tempAPI.ReleaseLevel("java", version) == "preview" {
				// Heuristic is wrong for Java (it should be stable). Add it.
				newAPI := serviceconfig.API{
					Path:      path,
					Languages: []string{"all"},
					ReleaseLevels: map[string]string{
						"java": "stable",
					},
				}
				apis = append(apis, newAPI)
				newPathsCount++
			}
			continue
		}

		// Already in sdk.yaml.
		_, hasJava := api.ReleaseLevels["java"]
		_, hasAll := api.ReleaseLevels["all"]
		if !hasJava && !hasAll {
			// No explicit release level. Check if heuristic aligns.
			if api.ReleaseLevel("java", version) == "preview" {
				// Heuristic would derive preview, but it should be stable. Add override.
				if api.ReleaseLevels == nil {
					api.ReleaseLevels = make(map[string]string)
				}
				api.ReleaseLevels["java"] = "stable"
				addedCount++
			}
		}
	}

	// 4. Sort APIs by path
	slices.SortFunc(apis, func(a, b serviceconfig.API) int {
		return strings.Compare(a.Path, b.Path)
	})

	// 5. Write back sdk.yaml
	if err := yaml.Write("internal/serviceconfig/sdk.yaml", apis); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write sdk.yaml: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("API paths with 'java: stable' added: %d\n", addedCount)
	fmt.Printf("New API paths added to sdk.yaml: %d\n", newPathsCount)
}
