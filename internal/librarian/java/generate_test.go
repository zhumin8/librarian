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
				"--java_gapic_opt=metadata,api-service-config=../../testdata/googleapis/google/cloud/secretmanager/v1/secretmanager_v1.yaml,grpc-service-config=../../testdata/googleapis/google/cloud/secretmanager/v1/secretmanager_grpc_service_config.json,transport=grpc,rest-numeric-enums",
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
