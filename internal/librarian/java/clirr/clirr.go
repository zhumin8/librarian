// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package clirr handles the generation of Clirr ignore files for Java libraries.
package clirr

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

//go:embed template/*.tmpl
var templatesFS embed.FS

var templates *template.Template

func init() {
	templates = template.Must(template.New("").ParseFS(templatesFS, "template/*.tmpl"))
}

// Generate generates the clirr-ignored-differences.xml file if it doesn't exist.
func Generate(protoModulePath string) error {
	outputPath := filepath.Join(protoModulePath, "clirr-ignored-differences.xml")
	if _, err := os.Stat(outputPath); err == nil {
		// File already exists, skip generation.
		return nil
	}

	protoPaths, err := findProtoPackages(protoModulePath)
	if err != nil {
		return fmt.Errorf("failed to find proto packages in %s: %w", protoModulePath, err)
	}

	if len(protoPaths) == 0 {
		return nil
	}

	data := struct {
		ProtoPaths []string
	}{
		ProtoPaths: protoPaths,
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return templates.ExecuteTemplate(f, "clirr-ignored-differences.xml.tmpl", data)
}

func findProtoPackages(protoModulePath string) ([]string, error) {
	srcDir := filepath.Join(protoModulePath, "src", "main", "java")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, nil
	}

	packageSet := make(map[string]bool)
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), "OrBuilder.java") {
			rel, err := filepath.Rel(srcDir, filepath.Dir(path))
			if err != nil {
				return err
			}
			// Use forward slashes for Clirr class names
			pkgPath := filepath.ToSlash(rel)
			if pkgPath != "" && pkgPath != "." {
				packageSet[pkgPath] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	packages := make([]string, 0, len(packageSet))
	for pkg := range packageSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	return packages, nil
}
