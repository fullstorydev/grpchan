// Command protoc-gen-grpchan is a protoc plugin that generates gRPC handler
// registration functions in Go that use grpchan.ServiceRegistry as the target
// instead of the *grpc.Server target that older gRPC generated code uses.
//
// This plugin is not needed if you are using protoc-gen-go-grpc v1.0.1 or
// newer and grpc-go v1.32.0 or newer.
//
// This plugin generates code that refers to a service description that is
// emitted by standard gRPC code generation. That package var was renamed to
// be exported in protoc-gen-go-grpc v1.1.0. So if you are using a plugin
// older than that (or using the older protoc-gen-go plugin, from the
// github.com/golang/protobuf repo), you must supply an option,
// "legacy_desc_names", to the plugin.
//
// This plugin can also generate client stub functions that use grpchan.Channel
// as the transport type, instead of *grpc.ClientConn. This is enabled by
// passing a "legacy_stubs" option to the plugin. This is needed to use the
// grpchan module if you are using the older protoc-gen-go plugin (from the
// github.com/golang/protobuf repo) and using a version older than v1.3.3.
package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	grpchanPackage protogen.GoImportPath = "github.com/fullstorydev/grpchan"
	grpcPackage    protogen.GoImportPath = "google.golang.org/grpc"
	contextPackage protogen.GoImportPath = "context"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
}

func run() error {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(in, req); err != nil {
		return err
	}
	resolveGoPackages(req)

	var args codeGenArgs
	gen, err := protogen.Options{ParamFunc: args.set}.New(req)
	if err != nil {
		return err
	}
	if err := generate(gen, args); err != nil {
		gen.Error(err)
	}
	out, err := proto.Marshal(gen.Response())
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

func generate(gen *protogen.Plugin, args codeGenArgs) error {
	gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL |
		pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
	gen.SupportedEditionsMinimum = descriptorpb.Edition_EDITION_PROTO2
	gen.SupportedEditionsMaximum = descriptorpb.Edition_EDITION_2024

	for _, f := range gen.Files {
		if f.Generate {
			generateChanStubs(gen, f, args)
		}
	}
	return nil
}

// resolveGoPackages adds M options to the request for the two behaviors that
// protogen does not provide, both of which this plugin has always supported:
//
//   - The "import_path" option, which sets the Go import path (and package
//     name) of every file to generate that does not have an M option.
//   - Files that have no go_package option and no M option, which protogen
//     rejects. For these, the package is named after the proto package and
//     the import path is the file's directory. This is how the older
//     protoc-gen-go plugin from github.com/golang/protobuf behaved.
func resolveGoPackages(req *pluginpb.CodeGeneratorRequest) {
	var importPath string
	mapped := map[string]bool{}
	for param := range strings.SplitSeq(req.GetParameter(), ",") {
		name, value, _ := strings.Cut(param, "=")
		switch {
		case name == "import_path":
			importPath = value
		case len(name) > 1 && name[0] == 'M':
			mapped[name[1:]] = true
		}
	}

	toGenerate := map[string]bool{}
	for _, name := range req.FileToGenerate {
		toGenerate[name] = true
	}

	var extra []string
	for _, fd := range req.ProtoFile {
		name := fd.GetName()
		switch {
		case mapped[name]:
		case importPath != "" && toGenerate[name]:
			extra = append(extra, "M"+name+"="+importPath+";"+cleanPackageName(path.Base(importPath)))
		case fd.GetOptions().GetGoPackage() == "":
			// protogen requires import paths to contain a '.' or '/'.
			extra = append(extra, "M"+name+"=./"+path.Dir(name)+";"+cleanPackageName(goPackageNameFor(fd)))
		}
	}
	if len(extra) > 0 {
		params := extra
		if req.GetParameter() != "" {
			params = append([]string{req.GetParameter()}, extra...)
		}
		req.Parameter = new(strings.Join(params, ","))
	}
}

func goPackageNameFor(fd *descriptorpb.FileDescriptorProto) string {
	name := fd.GetPackage()
	if name == "" {
		name = strings.TrimSuffix(path.Base(fd.GetName()), ".proto")
	}
	return name
}

func cleanPackageName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '_'
	}, name)
}

func generateChanStubs(gen *protogen.Plugin, f *protogen.File, args codeGenArgs) {
	if len(f.Services) == 0 {
		return
	}

	g := gen.NewGeneratedFile(f.GeneratedFilenamePrefix+".pb.grpchan.go", f.GoImportPath)
	g.P("// Code generated by protoc-gen-grpchan. DO NOT EDIT.")
	g.P("// source: ", f.Desc.Path())
	g.P()
	g.P("package ", f.GoPackageName)
	g.P()

	registry := g.QualifiedGoIdent(grpchanPackage.Ident("ServiceRegistry"))

	for _, sd := range f.Services {
		svcName := sd.GoName
		lowerSvcName := unexport(svcName)
		serviceDesc := serviceDescVarName(sd, args.legacyDescNames)

		g.P("// Deprecated: Use Register", svcName, "Server instead.")
		g.P("func RegisterHandler", svcName, "(reg ", registry, ", srv ", svcName, "Server) {")
		g.P("reg.RegisterService(&", serviceDesc, ", srv)")
		g.P("}")
		g.P()

		if !args.legacyStubs {
			continue
		}

		clientConn := g.QualifiedGoIdent(grpcPackage.Ident("ClientConnInterface"))
		callOption := g.QualifiedGoIdent(grpcPackage.Ident("CallOption"))
		context := g.QualifiedGoIdent(contextPackage.Ident("Context"))

		clientType := lowerSvcName + "ChannelClient"
		g.P("type ", clientType, " struct {")
		g.P("ch ", clientConn)
		g.P("}")
		g.P()

		g.P("// Deprecated: Use New", svcName, "Client instead.")
		g.P("func New", svcName, "ChannelClient(ch ", clientConn, ") ", svcName, "Client {")
		g.P("return &", clientType, "{ch: ch}")
		g.P("}")
		g.P()

		streamCount := 0
		for _, md := range sd.Methods {
			fullMethod := fmt.Sprintf("%q", fmt.Sprintf("/%s/%s", sd.Desc.FullName(), md.Desc.Name()))
			streamClient := svcName + "_" + md.GoName + "Client"
			streamClientImpl := lowerSvcName + md.GoName + "Client"
			input := "*" + g.QualifiedGoIdent(md.Input.GoIdent)
			output := "*" + g.QualifiedGoIdent(md.Output.GoIdent)

			switch {
			case md.Desc.IsStreamingClient():
				// bidi or client streaming method
				g.P("func (c *", clientType, ") ", md.GoName, "(ctx ", context, ", opts ...", callOption, ") (", streamClient, ", error) {")
				g.P("stream, err := c.ch.NewStream(ctx, &", serviceDesc, ".Streams[", streamCount, "], ", fullMethod, ", opts...)")
				g.P("if err != nil {")
				g.P("return nil, err")
				g.P("}")
				g.P("x := &", streamClientImpl, "{stream}")
				g.P("return x, nil")
				g.P("}")
				streamCount++
			case md.Desc.IsStreamingServer():
				// server streaming method
				g.P("func (c *", clientType, ") ", md.GoName, "(ctx ", context, ", in ", input, ", opts ...", callOption, ") (", streamClient, ", error) {")
				g.P("stream, err := c.ch.NewStream(ctx, &", serviceDesc, ".Streams[", streamCount, "], ", fullMethod, ", opts...)")
				g.P("if err != nil {")
				g.P("return nil, err")
				g.P("}")
				g.P("x := &", streamClientImpl, "{stream}")
				g.P("if err := x.ClientStream.SendMsg(in); err != nil {")
				g.P("return nil, err")
				g.P("}")
				g.P("if err := x.ClientStream.CloseSend(); err != nil {")
				g.P("return nil, err")
				g.P("}")
				g.P("return x, nil")
				g.P("}")
				streamCount++
			default:
				// unary method
				g.P("func (c *", clientType, ") ", md.GoName, "(ctx ", context, ", in ", input, ", opts ...", callOption, ") (", output, ", error) {")
				g.P("out := new(", g.QualifiedGoIdent(md.Output.GoIdent), ")")
				g.P("err := c.ch.Invoke(ctx, ", fullMethod, ", in, out, opts...)")
				g.P("if err != nil {")
				g.P("return nil, err")
				g.P("}")
				g.P("return out, nil")
				g.P("}")
			}
			g.P()
		}
	}
}

func serviceDescVarName(sd *protogen.Service, legacyNames bool) string {
	if legacyNames {
		return "_" + sd.GoName + "_serviceDesc"
	}
	return sd.GoName + "_ServiceDesc"
}

func unexport(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

type codeGenArgs struct {
	legacyStubs     bool
	legacyDescNames bool
}

// set handles the plugin options that protogen does not process itself. The
// "paths", "module", "import_path" and "M" options are handled by protogen.
func (a *codeGenArgs) set(name, value string) error {
	switch name {
	case "import_path":
		// handled by resolveGoPackages
		return nil
	case "debug":
		// Accepted for compatibility. It used to include unformatted source
		// in error messages; protogen now always includes the source.
		_, err := boolVal(name, value)
		return err
	case "legacy_stubs":
		v, err := boolVal(name, value)
		a.legacyStubs = v
		return err
	case "legacy_desc_names":
		v, err := boolVal(name, value)
		a.legacyDescNames = v
		return err
	default:
		return fmt.Errorf("unknown plugin option: %s", name)
	}
}

func boolVal(name, value string) (bool, error) {
	switch strings.ToLower(value) {
	case "", "true", "on", "yes", "1":
		// no value means "true"
		return true, nil
	case "false", "off", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean arg for option '%s': %s", name, value)
	}
}
