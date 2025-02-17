package shared

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/shared"
)

func KeyOrValueToInt(keyOrValue *proto.KeyOrValue, mf *proto.Manifest) int {
	if keyOrValue.GetKey() != "" {
		if mf.Config != nil {
			for _, item := range mf.Config.Items {
				if item.Name == keyOrValue.GetKey() {
					return int(item.GetIntValue())
				}
			}
		}
		root := shared.ReadRoot()
		if root.Config != nil {
			for _, item := range root.Config.Items {
				if item.Name == keyOrValue.GetKey() {
					return int(item.GetIntValue())
				}
			}
		}
	}
	return 0
}

func KeyOrValueToString(keyOrValue *proto.KeyOrValue, mf *proto.Manifest) string {
	if keyOrValue.GetKey() != "" {
		if mf.Config != nil {
			for _, item := range mf.Config.Items {
				if item.Name == keyOrValue.GetKey() {
					return item.GetStringValue()
				}
			}
		}
		root := shared.ReadRoot()
		if root.Config != nil {
			for _, item := range root.Config.Items {
				if item.Name == keyOrValue.GetKey() {
					return item.GetStringValue()
				}
			}
		}
	}
	return keyOrValue.GetValue()
}

type RPCInfo struct {
	Name           string
	InputType      string
	OutputType     string
	IsStreamInput  bool
	IsStreamOutput bool
}

func FindServiceInfo(rootDir, definition string) []RPCInfo {
	parts := strings.Split(definition, ":")
	pathSuffix := parts[0]
	serviceName := parts[1]

	bs, err := os.ReadFile(rootDir + "/" + pathSuffix)
	if err != nil {
		log.Fatal(err)
	}

	lines := strings.Split(string(bs), "\n")

	oneLiner := false
	serviceStartIdx := -1
	serviceEndIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "service "+serviceName) && strings.HasSuffix(line, "}") {
			oneLiner = true
			lines = []string{
				strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "service "+serviceName+" {"), "}")),
			}
		} else {
			if line == "service "+serviceName+" {" {
				serviceStartIdx = i
			}
			if serviceStartIdx >= 0 && line == "}" {
				serviceEndIdx = i
				break
			}
		}
	}

	var serviceBlock []string
	if !oneLiner {
		serviceBlock = filter(lines[serviceStartIdx+1:serviceEndIdx], func(x string) bool {
			return !strings.HasPrefix(strings.TrimSpace(x), "// ")
		})
	} else {
		serviceBlock = lines
	}

	var rpcs []string
	var inProgress string
	for _, line := range serviceBlock {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(inProgress) == "}" {
			inProgress = ""
		}

		if inProgress == "" {
			inProgress = strings.TrimSpace(line)
			if strings.HasSuffix(inProgress, ";") || strings.HasSuffix(inProgress, "{") {
				if strings.HasPrefix(inProgress, "rpc") {
					rpcs = append(rpcs, inProgress)
				}
				inProgress = ""
			}
		} else {
			inProgress += " " + strings.TrimSpace(line)
			if strings.HasSuffix(inProgress, ";") || strings.HasSuffix(inProgress, "{") {
				if strings.HasPrefix(inProgress, "rpc") {
					rpcs = append(rpcs, inProgress)
				}
				inProgress = ""
			}
		}
	}

	var rpcInfos []RPCInfo
	for _, rpc := range rpcs {
		firstBracket := strings.Index(rpc, "(")
		secondBracket := strings.Index(rpc, ")")
		thirdBracket := strings.LastIndex(rpc, "(")
		fourthBracket := strings.LastIndex(rpc, ")")

		arg := rpc[firstBracket+1 : secondBracket]
		ret := rpc[thirdBracket+1 : fourthBracket]

		rpcInfos = append(rpcInfos, RPCInfo{
			Name:           rpc[len("rpc "):firstBracket],
			InputType:      strings.TrimPrefix(arg, "stream "),
			IsStreamInput:  strings.HasPrefix(arg, "stream "),
			OutputType:     strings.TrimPrefix(ret, "stream "),
			IsStreamOutput: strings.HasPrefix(ret, "stream "),
		})
	}

	return rpcInfos
}

type ImportType uint

const (
	ImportTypeGo = ImportType(iota)
	ImportTypeExternal
	ImportTypeLocal
)

type Import struct {
	ImportAs   string
	Import     string
	ImportType ImportType
}

type NameTypePair struct {
	Name string
	Type string
}

func LoadTemplate(paths shared.Paths, name string) *template.Template {
	funcMap := template.FuncMap{
		"filter": func(imports []Import, t int) []Import {
			var filtered []Import
			for _, imp := range imports {
				if imp.ImportType != ImportType(t) {
					continue
				}
				filtered = append(filtered, imp)
			}
			return filtered
		},
		"isLast": func(i, l int) bool {
			return i >= l-1
		},
		"replace": func(input, from, to string) string {
			return strings.Replace(input, from, to, -1)
		},
	}
	bs, err := os.ReadFile(paths.RootDir + "/infra/generate/tmpls/" + name)
	if err != nil {
		log.Fatal(err)
	}
	tmpl, err := template.New(name).Funcs(funcMap).Parse(string(bs))
	if err != nil {
		log.Fatal(err)
	}
	return tmpl
}

func ImportPathForType(x string, repo string) string {
	importForType := map[string]string{
		"google.protobuf.Empty": "google.golang.org/protobuf/types/known/emptypb",
	}
	parts := strings.Split(x, ".")

	if parts[0] == repo {
		return fmt.Sprintf("github.com/magicpantry/%s/gen/%s", repo, strings.Join(parts[1:len(parts)-1], "/"))
	} else {
		return importForType[x]
	}
}

func BuildImports(paths shared.Paths, grpcServerManifest *proto.GrpcServer, rpcs []RPCInfo, repo string) []Import {
	serviceProtoPath := filepath.Dir(strings.Split(grpcServerManifest.Definition, ":")[0])
	importForType := map[string]string{
		"google.protobuf.Empty": "google.golang.org/protobuf/types/known/emptypb",
	}

	var imports []Import
	imports = append(
		imports,
		Import{
			Import:     fmt.Sprintf("github.com/magicpantry/%s/gen/%s", repo, serviceProtoPath),
			ImportAs:   "proto",
			ImportType: ImportTypeLocal,
		},
		Import{
			Import: fmt.Sprintf(
				"github.com/magicpantry/%s/%s/manifest",
				repo,
				shared.MakeRelativeToRoot(paths.GenDir, paths)),
			ImportType: ImportTypeLocal,
		})

	for _, rpc := range rpcs {
		if !rpc.IsStreamInput {
			imports = append(imports, Import{
				Import:     "context",
				ImportType: ImportTypeGo,
			})

			if strings.Contains(rpc.InputType, ".") {
				parts := strings.Split(rpc.InputType, ".")
				importAs := strings.ToLower(strings.Join(parts, "_"))

				if parts[0] == repo {
					imports = append(imports, Import{
						Import:     importAs,
						ImportType: ImportTypeLocal,
					})
				} else {
					imports = append(imports, Import{
						ImportAs:   importAs,
						Import:     importForType[rpc.InputType],
						ImportType: ImportTypeExternal,
					})
				}
			}
			if strings.Contains(rpc.OutputType, ".") {
				parts := strings.Split(rpc.OutputType, ".")
				importAs := strings.ToLower(strings.Join(parts, "_"))

				if parts[0] == repo {
					imports = append(imports, Import{
						Import:     importAs,
						ImportType: ImportTypeLocal,
					})
				} else {
					imports = append(imports, Import{
						ImportAs:   importAs,
						Import:     importForType[rpc.OutputType],
						ImportType: ImportTypeExternal,
					})
				}
			}
		}
	}

	return imports
}

func filter(xs []string, fn func(x string) bool) []string {
	var filtered []string
	for _, x := range xs {
		if !fn(x) {
			continue
		}
		filtered = append(filtered, x)
	}
	return filtered
}
