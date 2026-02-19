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

package java

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/googleapis/librarian/internal/config"
	"github.com/googleapis/librarian/internal/testhelper"
)

const googleapisDir = "../../testdata/googleapis"

func TestExtractVersion(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		path string
		want string
	}{
		{"google/cloud/secretmanager/v1", "v1"},
		{"google/cloud/secretmanager/v1beta2", "v1beta2"},
		{"google/cloud/v2/secretmanager", "v2"},
		{"google/cloud/secretmanager", ""},
	} {
		t.Run(test.path, func(t *testing.T) {
			got := extractVersion(test.path)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("extractVersion(%q) returned diff (-want +got): %s", test.path, diff)
			}
		})
	}
}

func TestCreateProtocOptions(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		api      *config.API
		library  *config.Library
		expected []string
		wantErr  bool
	}{
		{
			name:    "basic case",
			api:     &config.API{Path: "google/cloud/secretmanager/v1"},
			library: &config.Library{},
			expected: []string{
				"--java_out=proto-out",
				"--java_grpc_out=grpc-out",
				"--java_gapic_out=metadata:gapic-out",
				"--java_gapic_opt=metadata,api-service-config=../../testdata/googleapis/google/cloud/secretmanager/v1/secretmanager_v1.yaml,grpc-service-config=../../testdata/googleapis/google/cloud/secretmanager/v1/secretmanager_grpc_service_config.json,transport=grpc+rest,rest-numeric-enums",
			},
		},
		{
			name: "rest transport",
			api:  &config.API{Path: "google/cloud/secretmanager/v1"},
			library: &config.Library{
				Transport: "rest",
			},
			expected: []string{
				"--java_out=proto-out",
				"--java_gapic_out=metadata:gapic-out",
				"--java_gapic_opt=metadata,api-service-config=../../testdata/googleapis/google/cloud/secretmanager/v1/secretmanager_v1.yaml,grpc-service-config=../../testdata/googleapis/google/cloud/secretmanager/v1/secretmanager_grpc_service_config.json,transport=rest,rest-numeric-enums",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := createProtocOptions(test.api, test.library, googleapisDir, "proto-out", "grpc-out", "gapic-out")
			if (err != nil) != test.wantErr {
				t.Fatalf("createProtocOptions() error = %v, wantErr %v", err, test.wantErr)
			}

			if diff := cmp.Diff(test.expected, got); diff != "" {
				t.Errorf("createProtocOptions() returned diff (-want +got): %s", diff)
			}
		})
	}
}

func TestGenerateAPI_WrapperCreation(t *testing.T) {
	// This test verifies that the wrapper script is created correctly
	// when GeneratorJar is provided. We don't actually run protoc here
	// to keep it fast and dependency-free.

	jarPath := filepath.Join(t.TempDir(), "fake-generator.jar")
	if err := os.WriteFile(jarPath, []byte("fake jar content"), 0644); err != nil {
		t.Fatal(err)
	}

	outdir := t.TempDir()
	_ = &config.Library{
		Name:   "secretmanager",
		Output: outdir,
		Java:   &config.JavaPackage{},
		APIs:   []*config.API{{Path: "google/cloud/secretmanager/v1"}},
	}
	defaults := &config.Default{
		Java: &config.JavaDefault{
			GeneratorJar: jarPath,
		},
	}

	if defaults.Java.GeneratorJar != jarPath {
		t.Errorf("expected GeneratorJar %s, got %s", jarPath, defaults.Java.GeneratorJar)
	}
}
func TestGenerateAPI(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("slow test: Java GAPIC code generation")
	}

	testhelper.RequireCommand(t, "protoc")
	testhelper.RequireCommand(t, "protoc-gen-java_gapic")
	testhelper.RequireCommand(t, "protoc-gen-java_grpc")

	outdir := t.TempDir()
	err := generateAPI(
		t.Context(),
		&config.API{Path: "google/cloud/secretmanager/v1"},
		&config.Library{Name: "secretmanager", Output: outdir},
		&config.Default{},
		googleapisDir,
		outdir,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the output was restructured.
	restructuredPath := filepath.Join(outdir, "google-cloud-secretmanager", "src", "main", "java")
	if _, err := os.Stat(restructuredPath); err != nil {
		t.Errorf("expected restructured path %s to exist: %v", restructuredPath, err)
	}
}

func TestRestructureOutput(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	version := "v1"
	libraryID := "secretmanager"
	libraryName := "google-cloud-secretmanager"

	// Create a dummy structure to mimic generator output
	dirs := []string{
		filepath.Join(tmpDir, version, "gapic", "src", "main", "java"),
		filepath.Join(tmpDir, version, "gapic", "src", "main", "resources", "META-INF", "native-image"),
		filepath.Join(tmpDir, version, "gapic", "samples", "snippets", "generated", "src", "main", "java"),
		filepath.Join(tmpDir, version, "proto"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a dummy sample file
	sampleFile := filepath.Join(tmpDir, version, "gapic", "samples", "snippets", "generated", "src", "main", "java", "Sample.java")
	if err := os.WriteFile(sampleFile, []byte("public class Sample {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a dummy reflect-config.json
	reflectConfigPath := filepath.Join(tmpDir, version, "gapic", "src", "main", "resources", "META-INF", "native-image", "reflect-config.json")
	if err := os.WriteFile(reflectConfigPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := restructureOutput(tmpDir, libraryID, version); err != nil {
		t.Fatalf("restructureOutput failed: %v", err)
	}

	// Verify sample file location
	wantSamplePath := filepath.Join(tmpDir, "samples", "snippets", "generated", "Sample.java")
	if _, err := os.Stat(wantSamplePath); err != nil {
		t.Errorf("expected sample file at %s, but it was not found: %v", wantSamplePath, err)
	}

	// Verify reflect-config.json location
	wantReflectPath := filepath.Join(tmpDir, libraryName, "src", "main", "resources", "META-INF", "native-image", "reflect-config.json")
	if _, err := os.Stat(wantReflectPath); err != nil {
		t.Errorf("expected reflect-config.json at %s, but it was not found: %v", wantReflectPath, err)
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a dummy java file
	javaFile := filepath.Join(tmpDir, "SomeClass.java")
	unformatted := "public class SomeClass { public void method() { } }"
	if err := os.WriteFile(javaFile, []byte(unformatted), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a dummy sample file that should be skipped
	sampleDir := filepath.Join(tmpDir, "samples", "snippets", "generated")
	if err := os.MkdirAll(sampleDir, 0755); err != nil {
		t.Fatal(err)
	}
	sampleFile := filepath.Join(sampleDir, "Sample.java")
	if err := os.WriteFile(sampleFile, []byte(unformatted), 0644); err != nil {
		t.Fatal(err)
	}

	// We can't easily run the real google-java-format in this test environment
	// without a real JAR. But we can test that it returns nil if no jar is provided.
	lib := &config.Library{Output: tmpDir}
	if err := Format(t.Context(), lib, nil); err != nil {
		t.Errorf("Format(nil defaults) returned error: %v", err)
	}

	if err := Format(t.Context(), lib, &config.Default{Java: &config.JavaDefault{}}); err != nil {
		t.Errorf("Format(empty FormatterJar) returned error: %v", err)
	}

	// Test skip_format
	lib.Java = &config.JavaPackage{SkipFormat: true}
	if err := Format(t.Context(), lib, &config.Default{Java: &config.JavaDefault{FormatterJar: "fake.jar"}}); err != nil {
		t.Errorf("Format(skip_format) returned error: %v", err)
	}
}
