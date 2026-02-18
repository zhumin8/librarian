// Copyright 2026 Google LLC
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

package librarian

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// checkAndClean removes all files in dir except those in keep. The keep list
// should contain paths relative to dir. It returns an error if any file
// in keep does not exist.
func checkAndClean(dir string, keep []string) error {
	keepSet, err := check(dir, keep)
	if err != nil {
		return err
	}
	return clean(dir, keepSet)
}

// check validates the given directory and returns a set of files to keep.
// It ensures that the provided directory exists and is a directory.
// It also verifies that all files specified in 'keep' exist within 'dir'.
func check(dir string, keep []string) (map[string]bool, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot access output directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}
	keepSet := make(map[string]bool)
	for _, k := range keep {
		path := filepath.Join(dir, k)
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("keep file %q does not exist", k)
		}
		// Effectively get a canonical relative path. While in most cases
		// this will be equal to k, it might not be - in particular,
		// on Windows the directory separator in paths returned by Rel
		// will be a backslash.
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil, err
		}
		keepSet[rel] = true
	}
	return keepSet, nil
}

// clean removes files and directories from dir that are not in keepSet.
// NOTE: changed to remove dirs too because Java restructureoutput logic
func clean(dir string, keepSet map[string]bool) error {
	// First pass: remove all files that are not in keepSet.
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".github" || d.Name() == ".gemini" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "librarian.yaml" || keepSet[rel] {
			return nil
		}
		return os.Remove(path)
	})
	if err != nil {
		return err
	}

	// Second pass: remove empty directories that are not in keepSet.
	// We use filepath.Walk (not WalkDir) to easily handle post-order removal
	// by checking if the directory is empty after its children are processed.
	// Actually, WalkDir is fine too if we are careful.
	// Let's use a simple approach: walk the tree in reverse lexical order
	// to ensure we hit children before parents.
	var dirs []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".github" || d.Name() == ".gemini" {
				return filepath.SkipDir
			}
			if path != dir {
				dirs = append(dirs, path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Remove directories in reverse order (bottom-up).
	for i := len(dirs) - 1; i >= 0; i-- {
		d := dirs[i]
		rel, err := filepath.Rel(dir, d)
		if err != nil {
			return err
		}
		if keepSet[rel] {
			continue
		}
		// os.Remove only removes empty directories.
		// If it's not empty (because it contains kept files/dirs), it will fail with an error
		// which we can safely ignore if we only want to remove empty ones.
		_ = os.Remove(d)
	}

	return nil
}
