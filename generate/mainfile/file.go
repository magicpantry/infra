package mainfile

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

type MainTemplate struct {
	IsGRPC       bool
	IsJob        bool
	Imports      []shared.Import
	ServiceName  string
	ServiceFuncs []ServiceFunc
	InitLines    []string
	Clients      []Client
	Custom       string
}

func Build(paths infra_shared.Paths, mf *proto.Manifest, rpcs []shared.RPCInfo, repo string) string {
	addContext := false

	tmpl := shared.LoadTemplate(paths, "main.tmpl")
	mainTemplate := MainTemplate{
		IsGRPC: mf.Component.GetGrpcServer() != nil,
		IsJob:  mf.Component.GetJob() != nil,
	}

	if mf.Component.GetGrpcServer() != nil {
		mainTemplate.ServiceName = strings.Split(mf.Component.GetGrpcServer().Definition, ":")[1]
	}

	mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
		Import: fmt.Sprintf(
			"github.com/magicpantry/%s/gen/%s/manifest",
			repo,
			infra_shared.MakeRelativeToRoot(paths.ComponentDir, paths)),
		ImportType: shared.ImportTypeLocal,
	})
	mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
		Import: fmt.Sprintf(
			"github.com/magicpantry/%s/%s/handlers",
			repo,
			infra_shared.MakeRelativeToRoot(paths.ComponentDir, paths)),
		ImportType: shared.ImportTypeLocal,
	})
	mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
		Import:     "github.com/magicpantry/infra/shared/gcp",
		ImportType: shared.ImportTypeLocal,
	})
	if mf.Component.GetGrpcServer() != nil {
		mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
			Import:     "github.com/magicpantry/infra/shared/grpc",
			ImportType: shared.ImportTypeLocal,
		})
	} else if mf.Component.GetHttpServer() != nil {
		mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
			Import:     "net/http",
			ImportType: shared.ImportTypeGo,
		})
	} else if mf.Component.GetJob() != nil {
		addContext = true
		mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
			Import:     "log",
			ImportType: shared.ImportTypeGo,
		})
	}
	mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
		Import:     "context",
		ImportType: shared.ImportTypeGo,
	})
	if mf.Component.GetGrpcServer() != nil {
		mainTemplate.Imports = append(
			mainTemplate.Imports,
			shared.BuildImports(paths, mf.Component.GetGrpcServer(), rpcs, repo)...)
	}

	mainTemplate.ServiceFuncs = rpcsToServiceFuncs(mainTemplate.ServiceName, rpcs, false, "proto")

	root := infra_shared.ReadRootAtPath(paths.RootDir + "/root.textproto")
	rootConfigs := map[string]*proto.ConfigItem{}
	for _, item := range root.Config.Items {
		rootConfigs[item.Name] = item
	}

	mainTemplate.InitLines = append(
		mainTemplate.InitLines,
		fmt.Sprintf("mf.Repo = \"%s\"", repo))

	handleConfigItem := func(ci *proto.ConfigItem) {
		if ci.GetIntValue() != 0 {
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("mf.Config.%s = %d", ci.Name, ci.GetIntValue()))
		}
		if ci.GetDoubleValue() != 0.0 {
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("mf.Config.%s = %f", ci.Name, ci.GetDoubleValue()))
		}
		if ci.GetStringValue() != "" {
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("mf.Config.%s = \"%s\"", ci.Name, ci.GetStringValue()))
		}
		if ci.GetListValue() != nil {
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("mf.Config.%s = []string{%s}", ci.Name, join(ci.GetListValue().Values)))
		}
	}

	if mf.Config != nil {
		for _, config := range mf.Config.Items {
			if config.GetKeyValue() != nil {
				item, ok := rootConfigs[config.Name]
				if !ok {
					if config.GetKeyValue().Path == "" {
						log.Fatal("config item missing in root: " + config.Name)
					}
					local, ok := infra_shared.ReadConfigInOtherManifest(config.Name, paths.RootDir+"/"+config.GetKeyValue().Path)
					if !ok {
						log.Fatal("config item missing in root: " + config.Name)
					}
					item = local
				}
				handleConfigItem(item)
			} else {
				handleConfigItem(config)
			}
		}
	}

	addedNames := map[string]any{}
	for _, dep := range mf.RuntimeDependencies.Items {
		if dep.GetIp() != nil {
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "os",
				ImportType: shared.ImportTypeGo,
			})
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("mf.Dependencies.%s = os.Getenv(\"IP\")", dep.Name))
		}
		if dep.GetChan() != nil {
			for _, importInfo := range dep.GetChan().Type.Imports {
				mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
					ImportAs:   importInfo.ImportAs,
					Import:     importInfo.ImportName,
					ImportType: shared.ImportTypeLocal,
				})
			}
			if dep.GetChan().Size != nil {
				size := shared.KeyOrValueToInt(dep.GetChan().Size, mf)
				mainTemplate.InitLines = append(
					mainTemplate.InitLines,
					fmt.Sprintf("mf.Dependencies.%s = make(chan %s, %d)", dep.Name, dep.GetChan().Type.Name, size))
			} else {
				mainTemplate.InitLines = append(
					mainTemplate.InitLines,
					fmt.Sprintf("mf.Dependencies.%s = make(chan %s)", dep.Name, dep.GetChan().Type.Name))
			}
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("defer func() { close(mf.Dependencies.%s) }()", dep.Name))
		}
		if dep.GetCustom() != nil {
			addContext = true
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				ImportAs:   dep.GetCustom().ImportInfo.ImportAs,
				Import:     dep.GetCustom().ImportInfo.ImportName,
				ImportType: shared.ImportTypeLocal,
			})
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "log",
				ImportType: shared.ImportTypeGo,
			})
			mainTemplate.Custom = fmt.Sprintf(`go func() {
                                if err := %s.Run(ctx, mf); err != nil {
                                        log.Fatal(err)
                                }
                        }()`, dep.GetCustom().ImportInfo.ImportAs)
		}
		if dep.GetGenerativeModel() != nil {
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("mf.Dependencies.%s = gcp.NewGenerativeAIClient(ctx, \"%s\")", dep.Name, dep.GetGenerativeModel().Id))
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("defer mf.Dependencies.%s.Close()", dep.Name))
		}
		if dep.GetChrome() != nil {
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "github.com/chromedp/chromedp",
				ImportType: shared.ImportTypeExternal,
			})
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				"chromeContext, cf := chromedp.NewContext(ctx)")
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				"defer cf()")
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf(
					"mf.Dependencies.%s = chromeContext",
					dep.Name))
		}
		if dep.GetGrpcClientFactory() != nil {
			factory := dep.GetGrpcClientFactory()

			parts := strings.Split(factory.Definition, ":")

			path, name := parseImportParts(factory.Definition)
			importAs := strings.Join(popLast(strings.Split(strings.Split(path, ".")[0], "/")), "_")

			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "strings",
				ImportType: shared.ImportTypeGo,
			})
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				ImportAs:   importAs,
				Import:     fmt.Sprintf("github.com/magicpantry/%s/gen/%s", repo, strings.Join(popLast(strings.Split(path, "/")), "/")),
				ImportType: shared.ImportTypeLocal,
			})
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				ImportAs:   "proto_grpc",
				Import:     "google.golang.org/grpc",
				ImportType: shared.ImportTypeExternal,
			})
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/grpc",
				ImportType: shared.ImportTypeLocal,
			})
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "golang.org/x/oauth2",
				ImportType: shared.ImportTypeExternal,
			})
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "google.golang.org/grpc/metadata",
				ImportType: shared.ImportTypeExternal,
			})

			otherRPCs := shared.FindServiceInfo(paths.RootDir, factory.Definition)
			otherImports := shared.BuildImports(paths, &proto.GrpcServer{
				Definition: factory.Definition,
			}, otherRPCs, repo)
			for _, imp := range otherImports {
				mainTemplate.Imports = append(mainTemplate.Imports, imp)
			}

			if _, ok := addedNames[name]; !ok {
				addedNames[name] = struct{}{}
				mainTemplate.Clients = append(mainTemplate.Clients, Client{
					ImportAs:     importAs,
					ServiceName:  name,
					Secure:       true,
					ServiceFuncs: rpcsToServiceFuncs(name, otherRPCs, true, importAs),
				})
			}

			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf(
					"mf.Dependencies.%s = func(ctx context.Context, host string) (proto.%sClient, error) {",
					dep.Name, parts[1]),
				fmt.Sprintf("\tvar client proto.%sClient", parts[1]),
				fmt.Sprintf("\tlocal, err := grpc.ClientErr(host, proto.New%sClient)", parts[1]),
				"\tif err != nil {",
				"\t\treturn nil, err",
				"\t}",
				"\tclient = local",
				"\tif !strings.Contains(host, \":\") {",
				fmt.Sprintf("\t\twrapper := &%sClient{}", parts[1]),
				"\t\tts, err := grpc.TokenSourceErr(ctx, host)",
				"\t\tif err != nil {",
				"\t\t\treturn nil, err",
				"\t\t}",
				"\t\twrapper.Wrapped = client",
				"\t\twrapper.TokenSource = ts",
				"\t\tclient = wrapper",
				"\t}",
				"\treturn client, nil",
				"}")
		}
		if dep.GetGrpcClient() != nil {
			path, name := parseImportParts(dep.GetGrpcClient().Definition)
			importAs := strings.Join(popLast(strings.Split(strings.Split(path, ".")[0], "/")), "_")

			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				ImportAs:   importAs,
				Import:     fmt.Sprintf("github.com/magicpantry/%s/gen/%s", repo, strings.Join(popLast(strings.Split(path, "/")), "/")),
				ImportType: shared.ImportTypeLocal,
			})
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				ImportAs:   "proto_grpc",
				Import:     "google.golang.org/grpc",
				ImportType: shared.ImportTypeExternal,
			})
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/grpc",
				ImportType: shared.ImportTypeLocal,
			})
			if dep.GetGrpcClient().Secure {
				mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
					Import:     "golang.org/x/oauth2",
					ImportType: shared.ImportTypeExternal,
				})
				mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
					Import:     "google.golang.org/grpc/metadata",
					ImportType: shared.ImportTypeExternal,
				})
			}

			otherRPCs := shared.FindServiceInfo(paths.RootDir, dep.GetGrpcClient().Definition)

			if _, ok := addedNames[name]; !ok {
				addedNames[name] = struct{}{}
				mainTemplate.Clients = append(mainTemplate.Clients, Client{
					ImportAs:     importAs,
					ServiceName:  name,
					Secure:       dep.GetGrpcClient().Secure,
					ServiceFuncs: rpcsToServiceFuncs(name, otherRPCs, true, importAs),
				})
			}
			domain := shared.KeyOrValueToString(dep.GetGrpcClient().Domain, mf)
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf("%s_wrapped := grpc.Client(\"%s\", %s.New%sClient)", dep.Name, domain, importAs, name),
				fmt.Sprintf("%s_client := &%sClient{}", dep.Name, name),
				fmt.Sprintf("%s_client.Wrapped = %s_wrapped", dep.Name, dep.Name))

			if dep.GetGrpcClient().Secure {
				addContext = true
				mainTemplate.InitLines = append(
					mainTemplate.InitLines,
					fmt.Sprintf("%s_client.TokenSource = grpc.TokenSource(ctx, \"%s\")",
						dep.Name, domain))
			}

			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf(
					"mf.Dependencies.%s = %s_client",
					dep.Name, dep.Name))
		}
		if dep.GetBlockstore() != nil {
			addContext = true
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/blockstore",
				ImportType: shared.ImportTypeLocal,
			})
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				"sc := gcp.StorageClient(ctx)",
				"defer sc.Close()",
				fmt.Sprintf(
					"mf.Dependencies.%s = &blockstore.StorageBlockStore{Client: sc}",
					dep.Name))
		}
		if dep.GetDatabase() != nil {
			addContext = true
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/database",
				ImportType: shared.ImportTypeLocal,
			})
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf(
					"sc := gcp.SpannerClient(ctx, \"%s\")",
					dep.GetDatabase().Name),
				"defer sc.Close()",
				fmt.Sprintf(
					"mf.Dependencies.%s = &database.SpannerDatabase{Client: sc}",
					dep.Name))
		}
		if dep.GetDocstore() != nil {
			addContext = true
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/docstore",
				ImportType: shared.ImportTypeLocal,
			})
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				fmt.Sprintf(
					"fc := gcp.FirestoreClientWithDatabase(ctx, \"%s\")",
					dep.GetDocstore().Database),
				"defer fc.Close()",
				fmt.Sprintf(
					"mf.Dependencies.%s = &docstore.FirestoreDocStore{Client: fc}",
					dep.Name))
		}
		if dep.GetPool() != nil {
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import:     "github.com/magicpantry/infra/shared/pool",
				ImportType: shared.ImportTypeLocal,
			})
			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
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
			parts := strings.Split(dep.GetElastic().Implementation, ".")
			mainTemplate.Imports = append(mainTemplate.Imports, shared.Import{
				Import: fmt.Sprintf(
					"github.com/magicpantry/%s/%s",
					repo,
					strings.Join(parts[:len(parts)-1], "/")),
				ImportType: shared.ImportTypeLocal,
			})
			implementation := parts[len(parts)-1]
			var wrapped []string
			for _, u := range dep.GetElastic().Urls {
				wrapped = append(wrapped, fmt.Sprintf("\"%s\"", u))
			}

			mainTemplate.InitLines = append(
				mainTemplate.InitLines,
				"ec := gcp.ElasticClientWithParams(",
				fmt.Sprintf("\t[]string{%s},", strings.Join(wrapped, ", ")),
				fmt.Sprintf(
					"\t\"%s\", \"%s\")",
					dep.GetElastic().UsernameEnvKey, dep.GetElastic().PasswordEnvKey),
				fmt.Sprintf(
					"mf.Dependencies.%s = &textsearch.%s{Client: ec}",
					dep.Name, implementation))
		}
	}
	if addContext {
		mainTemplate.InitLines = append(
			[]string{"ctx := context.Background()"},
			mainTemplate.InitLines...)
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, mainTemplate); err != nil {
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
