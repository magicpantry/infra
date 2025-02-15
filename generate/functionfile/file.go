package functionfile

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/generate/shared"
	infra_shared "github.com/magicpantry/infra/shared"
)

type Client struct {
	ImportAs     string
	ServiceName  string
	ServiceFuncs []ServiceFunc
	Secure       bool
}

type ServiceFunc struct {
	Name string
	Args []shared.NameTypePair
	Rets []string
}

type FunctionTemplate struct {
	Imports   []shared.Import
	Namespace string
	Name      string
	InitLines []string
	Clients   []Client
}

func Build(paths infra_shared.Paths, mf *proto.Manifest, repo string) string {
	tmpl := shared.LoadTemplate(paths, "function.tmpl")
	functionTemplate := FunctionTemplate{
		Namespace: mf.Component.Namespace,
		Name:      mf.Component.Name,
	}

	functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
		Import: fmt.Sprintf(
			"github.com/magicpantry/%s/%s/handler",
            repo,
			infra_shared.MakeRelativeToRoot(paths.ComponentDir, paths)),
		ImportType: shared.ImportTypeLocal,
	})
	functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
		Import: fmt.Sprintf(
			"github.com/magicpantry/%s/gen/%s/manifest",
            repo,
			infra_shared.MakeRelativeToRoot(paths.ComponentDir, paths)),
		ImportType: shared.ImportTypeLocal,
	})
	functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
		Import:     "github.com/magicpantry/infra/shared/gcp",
		ImportType: shared.ImportTypeLocal,
	})
	functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
		Import:     "context",
		ImportType: shared.ImportTypeGo,
	})
	functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
		Import:     "github.com/cloudevents/sdk-go/v2/event",
		ImportType: shared.ImportTypeExternal,
	})
	functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
		Import:     "github.com/GoogleCloudPlatform/functions-framework-go/functions",
		ImportType: shared.ImportTypeExternal,
	})

	addContext := false
	if mf.Config != nil {
		for _, config := range mf.Config.Items {
			if config.GetIntValue() != 0 {
				functionTemplate.InitLines = append(
					functionTemplate.InitLines,
					fmt.Sprintf("mf.Config.%s = %d", config.Name, config.GetIntValue()))
			}
			if config.GetDoubleValue() != 0 {
				functionTemplate.InitLines = append(
					functionTemplate.InitLines,
					fmt.Sprintf("mf.Config.%s = %d", config.Name, config.GetDoubleValue()))
			}
			if config.GetStringValue() != "" {
				functionTemplate.InitLines = append(
					functionTemplate.InitLines,
					fmt.Sprintf("mf.Config.%s = \"%s\"", config.Name, config.GetStringValue()))
			}
			if config.GetListValue() != nil {
				functionTemplate.InitLines = append(
					functionTemplate.InitLines,
					fmt.Sprintf("mf.Config.%s = []string{%s}", config.Name, join(config.GetListValue().Values)))
			}
		}
	}
	for _, dep := range mf.RuntimeDependencies.Items {
		if dep.GetGrpcClient() != nil {
			path, name := parseImportParts(dep.GetGrpcClient().Definition)
			importAs := strings.Join(popLast(strings.Split(strings.Split(path, ".")[0], "/")), "_")

			functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
				ImportAs:   importAs,
				Import:     fmt.Sprintf("github.com/magicpantry/%s/gen/%s", repo, strings.Join(popLast(strings.Split(path, "/")), "/")),
				ImportType: shared.ImportTypeLocal,
			})
			functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
				ImportAs:   "proto_grpc",
				Import:     "google.golang.org/grpc",
				ImportType: shared.ImportTypeExternal,
			})
			functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/grpc",
				ImportType: shared.ImportTypeLocal,
			})
			if dep.GetGrpcClient().Secure {
				functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
					Import:     "golang.org/x/oauth2",
					ImportType: shared.ImportTypeExternal,
				})
				functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
					Import:     "google.golang.org/grpc/metadata",
					ImportType: shared.ImportTypeExternal,
				})
			}

			otherRPCs := shared.FindServiceInfo(paths.RootDir, dep.GetGrpcClient().Definition)

			functionTemplate.Clients = append(functionTemplate.Clients, Client{
				ImportAs:     importAs,
				ServiceName:  name,
				Secure:       dep.GetGrpcClient().Secure,
				ServiceFuncs: rpcsToServiceFuncs(name, otherRPCs, true, importAs),
			})
			domain := shared.KeyOrValueToString(dep.GetGrpcClient().Domain, mf)
			functionTemplate.InitLines = append(
				functionTemplate.InitLines,
				fmt.Sprintf("%s_wrapped := grpc.Client(\"%s\", %s.New%sClient)", dep.Name, domain, importAs, name),
				fmt.Sprintf("%s_client := &%sClient{}", dep.Name, name),
				fmt.Sprintf("%s_client.Wrapped = %s_wrapped", dep.Name, dep.Name))

			if dep.GetGrpcClient().Secure {
				addContext = true
				functionTemplate.InitLines = append(
					functionTemplate.InitLines,
					fmt.Sprintf("%s_client.TokenSource = grpc.TokenSource(ctx, \"%s\")",
						dep.Name, domain))
			}

			functionTemplate.InitLines = append(
				functionTemplate.InitLines,
				fmt.Sprintf(
					"mf.Dependencies.%s = %s_client",
					dep.Name, dep.Name))
		}
		if dep.GetBlockstore() != nil {
			addContext = true
			functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/blockstore",
				ImportType: shared.ImportTypeLocal,
			})
			functionTemplate.InitLines = append(
				functionTemplate.InitLines,
				"sc := gcp.StorageClient(ctx)",
				"defer sc.Close()",
				fmt.Sprintf(
					"mf.Dependencies.%s = &blockstore.StorageBlockStore{Client: sc}",
					dep.Name))
		}
		if dep.GetDocstore() != nil {
			addContext = true
			functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/docstore",
				ImportType: shared.ImportTypeLocal,
			})
			functionTemplate.InitLines = append(
				functionTemplate.InitLines,
				fmt.Sprintf(
					"fc := gcp.FirestoreClientWithDatabase(ctx, \"%s\")",
					dep.GetDocstore().Database),
				"defer fc.Close()",
				fmt.Sprintf(
					"mf.Dependencies.%s = &docstore.FirestoreDocStore{Client: fc}",
					dep.Name))
		}
		if dep.GetPool() != nil {
			functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/pool",
				ImportType: shared.ImportTypeLocal,
			})
			functionTemplate.InitLines = append(
				functionTemplate.InitLines,
				fmt.Sprintf("mf.Dependencies.%s = &pool.DocStorePool{", dep.Name),
				fmt.Sprintf(
					"\tDocStore:\tmf.Dependencies.%s,",
					dep.GetPool().DocstoreDependencyName),
				fmt.Sprintf("\tPoolName:\t\"%s\",", dep.GetPool().Collection),
				fmt.Sprintf(
					"\tLimit:\t\tmf.Config.%s,",
					dep.GetPool().LimitConfigName),
				"}")
		}
		if dep.GetElastic() != nil {
			functionTemplate.Imports = append(functionTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/textsearch",
				ImportType: shared.ImportTypeLocal,
			})

			parts := strings.Split(dep.GetElastic().Implementation, ".")
			implementation := parts[len(parts)-1]
			var wrapped []string
			for _, u := range dep.GetElastic().Urls {
				wrapped = append(wrapped, fmt.Sprintf("\"%s\"", u))
			}

			functionTemplate.InitLines = append(
				functionTemplate.InitLines,
				"ec := gcp.ElasticClientWithParams(",
				fmt.Sprintf("\t[]string{%s},", strings.Join(wrapped, ", ")),
				fmt.Sprintf(
					"\t\"%s\", \"%s\")",
					dep.GetElastic().UsernameEnvKey, dep.GetElastic().PasswordEnvKey),
				fmt.Sprintf(
					"mf.Dependencies.%s = &textsearch.%s{Client: ec}",
					dep.Name, implementation))
		}
		if dep.GetGrpcClientFactory() != nil {
			factory := dep.GetGrpcClientFactory()

			parts := strings.Split(factory.Definition, ":")

			functionTemplate.InitLines = append(
				functionTemplate.InitLines,
				fmt.Sprintf(
					"mf.Dependencies.%s = func(ctx context.Context, host string) (proto.%sClient, error) {",
					dep.Name, parts[1]),
				fmt.Sprintf("\treturn grpc.ClientErr(host, proto.New%sClient)", parts[1]),
				"}")
		}
	}
	if addContext {
		functionTemplate.InitLines = append(
			[]string{"ctx := context.Background()"},
			functionTemplate.InitLines...)
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, functionTemplate); err != nil {
		log.Fatal(err)
	}

	return string(buf.Bytes())
}

func join(xs []string) string {
	var wrapped []string
	for _, x := range xs {
		wrapped = append(wrapped, fmt.Sprintf("\"%s\"", x))
	}
	return strings.Join(wrapped, ", ")
}

func parseImportParts(x string) (string, string) {
	parts := strings.Split(x, ":")
	return parts[0], parts[1]
}

func popLast(xs []string) []string {
	return xs[:len(xs)-1]
}

func rpcsToServiceFuncs(serviceName string, rpcs []shared.RPCInfo, isClient bool, importAs string) []ServiceFunc {
	var sfs []ServiceFunc
	for _, rpc := range rpcs {
		sf := ServiceFunc{}
		sf.Name = rpc.Name

		if rpc.IsStreamInput {
			if isClient {
				sf.Args = append(sf.Args, shared.NameTypePair{
					Name: "ctx",
					Type: "context.Context",
				})
			}

			if !isClient {
				sf.Args = append(sf.Args, shared.NameTypePair{
					Name: "server",
					Type: fmt.Sprintf("%s.%s_%sServer", importAs, serviceName, rpc.Name),
				})
				sf.Rets = append(sf.Rets, "error")
			} else {
				sf.Rets = append(sf.Rets, fmt.Sprintf("%s.%s_%sClient", importAs, serviceName, rpc.Name))
				sf.Rets = append(sf.Rets, "error")
			}
		} else {
			inputType := fmt.Sprintf("%s.", importAs) + rpc.InputType
			outputType := fmt.Sprintf("%s.", importAs) + rpc.OutputType

			if strings.Contains(rpc.InputType, ".") {
				parts := strings.Split(rpc.InputType, ".")
				importAs := strings.ToLower(strings.Join(parts, "_"))
				inputType = importAs + "." + parts[len(parts)-1]
			}
			if strings.Contains(rpc.OutputType, ".") {
				parts := strings.Split(rpc.OutputType, ".")
				importAs := strings.ToLower(strings.Join(parts, "_"))
				outputType = importAs + "." + parts[len(parts)-1]
			}

			sf.Args = append(sf.Args, shared.NameTypePair{
				Name: "ctx",
				Type: "context.Context",
			})
			sf.Args = append(sf.Args, shared.NameTypePair{
				Name: "arg",
				Type: fmt.Sprintf("*%s", inputType),
			})

			sf.Rets = append(sf.Rets, fmt.Sprintf("*%s", outputType))
			sf.Rets = append(sf.Rets, "error")
		}

		if isClient {
			sf.Args = append(sf.Args, shared.NameTypePair{
				Name: "opts",
				Type: "...proto_grpc.CallOption",
			})
		}

		sfs = append(sfs, sf)
	}
	return sfs
}
