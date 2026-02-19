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

// Package java provides Java specific functionality for librarian.
package java

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/googleapis/librarian/internal/config"
	"github.com/googleapis/librarian/internal/serviceconfig"
)

// GenerateLibraries generates all the given libraries in sequence.
func GenerateLibraries(ctx context.Context, libraries []*config.Library, defaults *config.Default, googleapisDir string) error {
	for _, library := range libraries {
		if err := generate(ctx, library, defaults, googleapisDir); err != nil {
			return err
		}
	}
	return nil
}

// generate generates a Java client library.
func generate(ctx context.Context, library *config.Library, defaults *config.Default, googleapisDir string) error {
	if len(library.APIs) == 0 {
		return fmt.Errorf("no apis configured for library %q", library.Name)
	}

	outdir, err := filepath.Abs(library.Output)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory path: %w", err)
	}

	// Ensure googleapisDir is absolute to avoid issues with relative paths in protoc.
	googleapisDir, err = filepath.Abs(googleapisDir)
	if err != nil {
		return fmt.Errorf("failed to resolve googleapis directory path: %w", err)
	}

	if err := os.MkdirAll(outdir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, api := range library.APIs {
		if err := generateAPI(ctx, api, library, defaults, googleapisDir, outdir); err != nil {
			return fmt.Errorf("failed to generate api %q: %w", api.Path, err)
		}
	}

	return nil
}

func generateAPI(ctx context.Context, api *config.API, library *config.Library, defaults *config.Default, googleapisDir, outdir string) error {
	version := extractVersion(api.Path)
	if version == "" {
		return fmt.Errorf("failed to extract version from api path %q", api.Path)
	}

	// Output directories for Java as seen in v0.7.0
	gapicDir := filepath.Join(outdir, version, "gapic")
	grpcDir := filepath.Join(outdir, version, "grpc")
	protoDir := filepath.Join(outdir, version, "proto")

	for _, dir := range []string{gapicDir, grpcDir, protoDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	protocOptions, err := createProtocOptions(api, library, googleapisDir, protoDir, grpcDir, gapicDir)
	if err != nil {
		return err
	}

	apiDir := filepath.Join(googleapisDir, api.Path)
	protos, err := filepath.Glob(apiDir + "/*.proto")
	if err != nil {
		return fmt.Errorf("failed to find protos: %w", err)
	}
	if len(protos) == 0 {
		return fmt.Errorf("no protos found in api %q", api.Path)
	}
	protos = append(protos, filepath.Join(googleapisDir, "google", "cloud", "common_resources.proto"))

	cmdArgs := []string{"protoc", "--experimental_allow_proto3_optional", "-I=" + googleapisDir}
	cmdArgs = append(cmdArgs, protos...)
	cmdArgs = append(cmdArgs, protocOptions...)

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	// If plugins are provided in defaults, create a temporary directory and add it to PATH.
	if defaults != nil && defaults.Java != nil && (defaults.Java.GeneratorJar != "" || defaults.Java.GRPCPlugin != "") {
		tmpDir, err := os.MkdirTemp("", "librarian-java-plugin-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		if defaults.Java.GeneratorJar != "" {
			jarPath, err := filepath.Abs(defaults.Java.GeneratorJar)
			if err != nil {
				return err
			}
			wrapperPath := filepath.Join(tmpDir, "protoc-gen-java_gapic")
			wrapperContent := fmt.Sprintf("#!/bin/bash\nset -e\nexec java -cp %q com.google.api.generator.Main \"$@\"\n", jarPath)
			if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0755); err != nil {
				return err
			}
		}

		if defaults.Java.GRPCPlugin != "" {
			pluginPath, err := filepath.Abs(defaults.Java.GRPCPlugin)
			if err != nil {
				return err
			}
			wrapperPath := filepath.Join(tmpDir, "protoc-gen-java_grpc")
			wrapperContent := fmt.Sprintf("#!/bin/bash\nset -e\nexec %q \"$@\"\n", pluginPath)
			if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0755); err != nil {
				return err
			}
		}

		cmd.Env = append(os.Environ(), "PATH="+tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd.String(), err)
	}

	// Unzip the temp-codegen.srcjar.
	srcjarPath := filepath.Join(gapicDir, "temp-codegen.srcjar")
	if _, err := os.Stat(srcjarPath); err == nil {
		if err := unzip(srcjarPath, gapicDir); err != nil {
			return fmt.Errorf("failed to unzip %s: %w", srcjarPath, err)
		}
	}

	if err := restructureOutput(outdir, library.Name, version); err != nil {
		return fmt.Errorf("failed to restructure output: %w", err)
	}

	// Cleanup intermediate version directory
	if err := os.RemoveAll(filepath.Join(outdir, version)); err != nil {
		return fmt.Errorf("failed to cleanup intermediate files: %w", err)
	}

	return nil
}

func createProtocOptions(api *config.API, library *config.Library, googleapisDir, protoDir, grpcDir, gapicDir string) ([]string, error) {
	args := []string{
		fmt.Sprintf("--java_out=%s", protoDir),
	}

	transport := library.Transport
	if transport == "" {
		transport = "grpc" // Default to grpc
	}

	if transport != "rest" {
		args = append(args, fmt.Sprintf("--java_grpc_out=%s", grpcDir))
	}

	gapicOpts := []string{"metadata"}

	sc, err := serviceconfig.Find(googleapisDir, api.Path, serviceconfig.LangJava)
	if err != nil {
		return nil, err
	}
	if sc != nil && sc.ServiceConfig != "" {
		gapicOpts = append(gapicOpts, fmt.Sprintf("api-service-config=%s", filepath.Join(googleapisDir, sc.ServiceConfig)))
	}

	gc, err := serviceconfig.FindGRPCServiceConfig(googleapisDir, api.Path)
	if err != nil {
		return nil, err
	}
	if gc != "" {
		gapicOpts = append(gapicOpts, fmt.Sprintf("grpc-service-config=%s", filepath.Join(googleapisDir, gc)))
	}

	gapicOpts = append(gapicOpts, fmt.Sprintf("transport=%s", transport))

	// rest-numeric-enums
	gapicOpts = append(gapicOpts, "rest-numeric-enums")

	args = append(args, fmt.Sprintf("--java_gapic_out=metadata:%s", gapicDir))
	args = append(args, "--java_gapic_opt="+strings.Join(gapicOpts, ","))

	return args, nil
}

func extractVersion(path string) string {
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "v") {
			return parts[i]
		}
	}
	return ""
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		rc.Close()
		closeErr := outFile.Close()

		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func restructureOutput(outputDir, libraryID, version string) error {
	gapicSrcDir := filepath.Join(outputDir, version, "gapic", "src", "main", "java")
	gapicTestDir := filepath.Join(outputDir, version, "gapic", "src", "test", "java")
	protoSrcDir := filepath.Join(outputDir, version, "proto")
	resourceNameSrcDir := filepath.Join(outputDir, version, "gapic", "proto", "src", "main", "java")
	samplesDir := filepath.Join(outputDir, version, "gapic", "samples", "snippets", "generated", "src", "main", "java")

	// Adjusting libraryID for Java naming convention as seen in v0.7.0
	libraryName := libraryID
	if !strings.HasPrefix(libraryName, "google-cloud-") {
		libraryName = "google-cloud-" + libraryID
	}

	gapicDestDir := filepath.Join(outputDir, libraryName, "src", "main", "java")
	gapicTestDestDir := filepath.Join(outputDir, libraryName, "src", "test", "java")
	protoDestDir := filepath.Join(outputDir, fmt.Sprintf("proto-%s-%s", libraryName, version), "src", "main", "java")
	resourceNameDestDir := filepath.Join(outputDir, fmt.Sprintf("proto-%s-%s", libraryName, version), "src", "main", "java")
	grpcDestDir := filepath.Join(outputDir, fmt.Sprintf("grpc-%s-%s", libraryName, version), "src", "main", "java")
	samplesDestDir := filepath.Join(outputDir, "samples", "snippets", "generated")

	destDirs := []string{gapicDestDir, gapicTestDestDir, protoDestDir, samplesDestDir, grpcDestDir}
	for _, dir := range destDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Remove the location classes from the proto output to avoid conflicts
	os.RemoveAll(filepath.Join(protoSrcDir, "com", "google", "cloud", "location"))
	os.Remove(filepath.Join(protoSrcDir, "google", "cloud", "CommonResources.java"))

	moves := map[string]string{
		protoSrcDir: protoDestDir,
		filepath.Join(outputDir, version, "grpc"): grpcDestDir,
	}
	for src, dest := range moves {
		if _, err := os.Stat(src); err == nil {
			if err := moveAndMerge(src, dest); err != nil {
				return err
			}
		}
	}

	if err := copyAndMerge(gapicSrcDir, gapicDestDir); err != nil {
		return err
	}
	if err := copyAndMerge(gapicTestDir, gapicTestDestDir); err != nil {
		return err
	}
	if err := copyAndMerge(samplesDir, samplesDestDir); err != nil {
		return err
	}
	if err := copyAndMerge(resourceNameSrcDir, resourceNameDestDir); err != nil {
		return err
	}

	return nil
}

func moveAndMerge(sourceDir, targetDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		oldPath := filepath.Join(sourceDir, entry.Name())
		newPath := filepath.Join(targetDir, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(newPath, 0755); err != nil {
				return err
			}
			if err := moveAndMerge(oldPath, newPath); err != nil {
				return err
			}
		} else {
			if err := os.Rename(oldPath, newPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyAndMerge(src, dest string) error {
	entries, err := os.ReadDir(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			if err := copyAndMerge(srcPath, destPath); err != nil {
				return err
			}
		} else {
			if err := os.Rename(srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}
