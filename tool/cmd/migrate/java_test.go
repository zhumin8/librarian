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

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRefactorOwlBot(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "standard template",
			content: `import synthtool as s
from synthtool.languages import java

for library in s.get_staging_dirs():
    s.move(library)

s.remove_staging_dirs()
java.common_templates(monorepo=True, excludes=[
    ".github/*",
    ".gitignore"
])
`,
			want: `import synthtool as s


for library in s.get_staging_dirs():
    s.move(library)
`,
		},
		{
			name: "customized template with java.remove_method",
			content: `import synthtool as s
from synthtool.languages import java

for library in s.get_staging_dirs():
    java.remove_method('DataSourceName.java', 'public static List<DataSourceName> parseList')
    s.move(library)

s.remove_staging_dirs()
java.common_templates(monorepo=True, excludes=[".github/*"])
`,
			want: `import synthtool as s
from synthtool.languages import java

for library in s.get_staging_dirs():
    java.remove_method('DataSourceName.java', 'public static List<DataSourceName> parseList')
    s.move(library)
`,
		},
		{
			name: "java-notification special case",
			content: `import synthtool.languages.java as java

AUTOSYNTH_MULTIPLE_COMMITS = True

java.common_templates(monorepo=True, excludes=[".github/*"])
`,
			want: `AUTOSYNTH_MULTIPLE_COMMITS = True
`,
		},
		{
			name: "customized with os.remove",
			content: `import synthtool as s
from synthtool.languages import java
import os

for library in s.get_staging_dirs():
    os.remove("SomeProto.java")
    s.move(library)

s.remove_staging_dirs()
java.common_templates(monorepo=True)
`,
			want: `import synthtool as s

import os

for library in s.get_staging_dirs():
    os.remove("SomeProto.java")
    s.move(library)
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := refactorOwlBot(test.content)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("refactorOwlBot() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
