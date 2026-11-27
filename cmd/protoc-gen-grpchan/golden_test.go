package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// The descriptor set is checked in so that these tests do not need protoc.
//go:generate protoc -I testdata --include_imports --descriptor_set_out=testdata/descriptors.pb testdata/services.proto testdata/noservices.proto testdata/nogopackage.proto testdata/sub/nested.proto

var update = flag.Bool("update", false, "rewrite golden files in testdata/golden")

const runPluginEnv = "PROTOC_GEN_GRPCHAN_TEST_RUN_PLUGIN"

func TestMain(m *testing.M) {
	// Re-exec of the test binary acts as the plugin, so the tests exercise
	// the real main function, argument parsing and stdin/stdout protocol.
	if os.Getenv(runPluginEnv) == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

type goldenCase struct {
	name  string
	param string
	files []string
}

var goldenCases = []goldenCase{
	{
		name:  "default",
		files: []string{"services.proto", "noservices.proto"},
	},
	{
		name:  "legacy_stubs",
		param: "legacy_stubs",
		files: []string{"services.proto", "noservices.proto"},
	},
	{
		name:  "legacy_stubs_explicit_false",
		param: "legacy_stubs=false",
		files: []string{"services.proto"},
	},
	{
		name:  "legacy_desc_names",
		param: "legacy_desc_names=true,legacy_stubs=true",
		files: []string{"services.proto"},
	},
	{
		name:  "source_relative",
		param: "paths=source_relative,legacy_stubs",
		files: []string{"services.proto", "noservices.proto"},
	},
	{
		name:  "module",
		param: "module=example.com/grpchan/testdata,legacy_stubs",
		files: []string{"services.proto", "noservices.proto"},
	},
	{
		name:  "import_map",
		param: "Mcommon/types.proto=example.com/other/commonpb;otherpb,Mservices.proto=example.com/mapped;mappedpb,legacy_stubs",
		files: []string{"services.proto"},
	},
	{
		name:  "import_path",
		param: "import_path=example.com/override/pkg,legacy_stubs",
		files: []string{"nogopackage.proto"},
	},
	{
		name:  "import_map_no_go_package",
		param: "Mnogopackage.proto=example.com/mapped/nogopackage;nogopackagepb,Mcommon/types.proto=example.com/mapped/common;commonpb,legacy_stubs",
		files: []string{"nogopackage.proto"},
	},
	{
		name:  "no_go_package",
		param: "legacy_stubs",
		files: []string{"nogopackage.proto", "sub/nested.proto"},
	},
	{
		name:  "no_go_package_source_relative",
		param: "legacy_stubs,paths=source_relative",
		files: []string{"nogopackage.proto", "sub/nested.proto"},
	},
	{
		name:  "no_go_package_with_import_path",
		param: "legacy_stubs,import_path=example.com/override/pkg",
		files: []string{"nogopackage.proto", "sub/nested.proto"},
	},
	{
		name:  "import_path_overrides_go_package",
		param: "import_path=example.com/override/pkg,legacy_stubs",
		files: []string{"services.proto"},
	},
	{
		name:  "import_path_two_files",
		param: "import_path=example.com/override/pkg,legacy_stubs",
		files: []string{"services.proto", "nogopackage.proto"},
	},
	{
		name:  "import_path_with_map",
		param: "import_path=example.com/override/pkg,Mservices.proto=example.com/mapped;mappedpb,legacy_stubs",
		files: []string{"services.proto", "nogopackage.proto"},
	},
	{
		name:  "multiple_files_same_package_options",
		param: "legacy_stubs,legacy_desc_names",
		files: []string{"services.proto", "nogopackage.proto"},
	},
}

var errorCases = []goldenCase{
	{name: "unknown_option", param: "bogus=1", files: []string{"services.proto"}},
	{name: "bad_bool", param: "legacy_stubs=maybe", files: []string{"services.proto"}},
	{name: "bad_paths", param: "paths=bogus", files: []string{"services.proto"}},
	{name: "module_and_source_relative", param: "module=example.com/x,paths=source_relative", files: []string{"services.proto"}},
}

func loadDescriptors(t *testing.T) []*descriptorpb.FileDescriptorProto {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "descriptors.pb"))
	if err != nil {
		t.Fatalf("reading descriptor set: %v", err)
	}
	var set descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &set); err != nil {
		t.Fatalf("unmarshaling descriptor set: %v", err)
	}
	return set.File
}

func runPlugin(t *testing.T, protoFiles []*descriptorpb.FileDescriptorProto, tc goldenCase) *pluginpb.CodeGeneratorResponse {
	t.Helper()
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: tc.files,
		ProtoFile:      protoFiles,
	}
	if tc.param != "" {
		req.Parameter = proto.String(tc.param)
	}
	in, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("finding test executable: %v", err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), runPluginEnv+"=1")
	cmd.Stdin = bytes.NewReader(in)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var resp pluginpb.CodeGeneratorResponse
	if err := proto.Unmarshal(stdout.Bytes(), &resp); err != nil || (stdout.Len() == 0 && runErr != nil) {
		// Plugins may report failure by exiting non-zero, which is
		// equivalent to protoc.
		resp.Reset()
		resp.Error = proto.String("plugin failed: " + runErr.Error() + ": " + stderr.String())
	}
	return &resp
}

func TestGolden(t *testing.T) {
	protoFiles := loadDescriptors(t)
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := runPlugin(t, protoFiles, tc)
			if resp.Error != nil {
				t.Fatalf("plugin error: %s", resp.GetError())
			}
			got := map[string]string{}
			for _, f := range resp.File {
				if f.InsertionPoint != nil {
					t.Fatalf("unexpected insertion point in %s", f.GetName())
				}
				got[f.GetName()] = f.GetContent()
			}
			checkGolden(t, tc.name, got)
		})
	}
}

func TestErrors(t *testing.T) {
	protoFiles := loadDescriptors(t)
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := runPlugin(t, protoFiles, tc)
			if resp.Error == nil {
				t.Fatalf("expected an error, but got %d output files", len(resp.File))
			}
			if len(resp.File) != 0 {
				t.Fatalf("expected no output files when there is an error, got %d", len(resp.File))
			}
		})
	}
}

func checkGolden(t *testing.T, name string, got map[string]string) {
	t.Helper()
	dir := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		for fn, content := range got {
			p := filepath.Join(dir, filepath.FromSlash(fn))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}

	want := map[string]string{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		want[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("reading golden files: %v", err)
	}

	names := func(m map[string]string) []string {
		var s []string
		for k := range m {
			s = append(s, k)
		}
		sort.Strings(s)
		return s
	}
	if g, w := strings.Join(names(got), ", "), strings.Join(names(want), ", "); g != w {
		t.Fatalf("output files differ:\n got: %s\nwant: %s", g, w)
	}
	for _, fn := range names(want) {
		if got[fn] != want[fn] {
			t.Errorf("content of %s differs from golden file (run with -update to refresh):\n--- got ---\n%s\n--- want ---\n%s", fn, got[fn], want[fn])
		}
	}
}
