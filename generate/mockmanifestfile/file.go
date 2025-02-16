package mockmanifestfile

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"log"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/generate/shared"
	infra_shared "github.com/magicpantry/infra/shared"
)

func Build(paths infra_shared.Paths, mf *proto.Manifest, rpcs []shared.RPCInfo, repo string) string {
	fs := token.NewFileSet()

	var imports []ast.Spec
	var mockedFields []*ast.Field
	var statements []ast.Stmt

	imports = append(imports, &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: wrap("context"),
		},
	})
	imports = append(imports, &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: wrap("testing"),
		},
	})
	imports = append(imports, &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: wrap(fmt.Sprintf("github.com/magicpantry/%s/%s/manifest", repo, infra_shared.MakeRelativeToRoot(paths.GenDir, paths))),
		},
	})

	mockedFields = append(mockedFields, &ast.Field{
		Names: []*ast.Ident{&ast.Ident{Name: "Context"}},
		Type:  &ast.Ident{Name: "context.Context"},
	})

	statements = append(statements, &ast.AssignStmt{
		Tok: token.DEFINE,
		Lhs: []ast.Expr{&ast.Ident{Name: "mockManifest"}},
		Rhs: []ast.Expr{&ast.Ident{Name: "&MockManifest{}"}},
	})
	statements = append(statements, &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []ast.Expr{&ast.Ident{Name: "mockManifest.Mocked"}},
		Rhs: []ast.Expr{&ast.Ident{Name: "&Mocked{}"}},
	})
	statements = append(statements, &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []ast.Expr{&ast.Ident{Name: "mockManifest.Manifest"}},
		Rhs: []ast.Expr{&ast.Ident{Name: "&manifest.Manifest{}"}},
	})
	statements = append(statements, &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []ast.Expr{&ast.Ident{Name: "mockManifest.Manifest.Config"}},
		Rhs: []ast.Expr{&ast.Ident{Name: "manifest.Config{}"}},
	})
	statements = append(statements, &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []ast.Expr{&ast.Ident{Name: "mockManifest.Manifest.Dependencies"}},
		Rhs: []ast.Expr{&ast.Ident{Name: "manifest.Dependencies{}"}},
	})
	statements = append(statements, &ast.AssignStmt{
		Tok: token.ASSIGN,
		Lhs: []ast.Expr{&ast.Ident{Name: "mockManifest.Mocked.Context"}},
		Rhs: []ast.Expr{&ast.Ident{Name: "context.Background()"}},
	})

	decls := []ast.Decl{
		&ast.GenDecl{
			Tok: token.TYPE,
			Specs: []ast.Spec{
				&ast.TypeSpec{
					Name: &ast.Ident{Name: "MockManifest"},
					Type: &ast.StructType{
						Fields: &ast.FieldList{
							List: []*ast.Field{
								&ast.Field{
									Names: []*ast.Ident{&ast.Ident{Name: "Manifest"}},
									Type:  &ast.Ident{Name: "*manifest.Manifest"},
								},
								&ast.Field{
									Names: []*ast.Ident{&ast.Ident{Name: "Mocked"}},
									Type:  &ast.Ident{Name: "*Mocked"},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, rpc := range rpcs {
		if rpc.IsStreamInput || rpc.IsStreamOutput {
			inputPath, inputName := parseImportPartsDot(rpc.InputType)
			outputPath, outputName := parseImportPartsDot(rpc.OutputType)

			inputImportAs := strings.Join(popFirst(strings.Split(inputPath, ".")), "_")
			outputImportAs := strings.Join(popFirst(strings.Split(outputPath, ".")), "_")

			imports = append(imports, &ast.ImportSpec{
				Name: &ast.Ident{Name: inputImportAs},
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: wrap(shared.ImportPathForType(rpc.InputType, repo)),
				},
			})
			imports = append(imports, &ast.ImportSpec{
				Name: &ast.Ident{Name: outputImportAs},
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: wrap(shared.ImportPathForType(rpc.OutputType, repo)),
				},
			})
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{
					Name: fmt.Sprintf("%sServer", rpc.Name),
				}},
				Type: &ast.Ident{Name: fmt.Sprintf(
					"*mocks.Server[*%s.%s, *%s.%s]",
					inputImportAs, inputName,
					outputImportAs, outputName,
				)},
			})
		} else {
			inputType := rpc.InputType
			inputPath, inputName := parseImportPartsDot(inputType)
			inputImportAs := strings.Join(popFirst(strings.Split(inputPath, ".")), "_")

			if inputImportAs == "" {
				protoPath := strings.Split(mf.Component.GetGrpcServer().Definition, ":")[0]
				protoParts := strings.Split(protoPath, "/")
				inputType = repo + "." + strings.Join(protoParts[:len(protoParts)-1], ".") + "." + inputName

				inputPath, inputName = parseImportPartsDot(inputType)
				inputImportAs = strings.Join(popFirst(strings.Split(inputPath, ".")), "_")
			}

			imports = append(imports, &ast.ImportSpec{
				Name: &ast.Ident{Name: inputImportAs},
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: wrap(shared.ImportPathForType(inputType, repo)),
				},
			})

			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{
					Name: fmt.Sprintf("%sRequest", rpc.Name),
				}},
				Type: &ast.Ident{Name: fmt.Sprintf(
					"*%s.%s",
					inputImportAs, inputName,
				)},
			})
		}
	}
	for _, dep := range mf.RuntimeDependencies.Items {
		if dep.GetGenerativeModel() != nil {
			imports = append(imports, &ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: wrap("github.com/magicpantry/infra/shared/genai"),
				},
			})
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{
					Name: fmt.Sprintf("%s", dep.Name),
				}},
				Type: &ast.Ident{Name: "genai.GenAI"},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
			})
		}
		if dep.GetChrome() != nil {
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{
					Name: fmt.Sprintf("%s", dep.Name),
				}},
				Type: &ast.Ident{Name: "context.Context"},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: "context.Background()"}},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
			})
		}
		if dep.GetGrpcClient() != nil {
			path, name := parseImportParts(dep.GetGrpcClient().Definition)
			importAs := strings.Join(popLast(strings.Split(strings.Split(path, ".")[0], "/")), "_")
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("&%sClient{}", dep.Name)}},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
			})
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: dep.Name}},
				Type:  &ast.Ident{Name: fmt.Sprintf("*%sClient", dep.Name)},
			})
			imports = append(imports, &ast.ImportSpec{
				Name: &ast.Ident{Name: importAs},
				Path: &ast.BasicLit{
					Kind: token.STRING,
					Value: wrap(fmt.Sprintf(
						"github.com/magicpantry/%s/gen/%s",
						repo,
						parseImportPath(path))),
				},
			})

			fields := []*ast.Field{
				&ast.Field{
					Names: []*ast.Ident{&ast.Ident{Name: "ReturnErr"}},
					Type:  &ast.Ident{Name: "bool"},
				},
			}

			otherRPCs := shared.FindServiceInfo(paths.RootDir, dep.GetGrpcClient().Definition)
			for _, rpc := range otherRPCs {
				inputType := rpc.InputType
				inputPath, inputName := parseImportPartsDot(inputType)
				inputImportAs := strings.Join(popFirst(strings.Split(inputPath, ".")), "_")

				if inputImportAs == "" {
					protoPath := strings.Split(dep.GetGrpcClient().Definition, ":")[0]
					protoParts := strings.Split(protoPath, "/")
					inputType = repo + "." + strings.Join(protoParts[:len(protoParts)-1], ".") + "." + inputName

					inputPath, inputName = parseImportPartsDot(inputType)
					inputImportAs = strings.Join(popFirst(strings.Split(inputPath, ".")), "_")
				}
				outputType := rpc.OutputType
				outputPath, outputName := parseImportPartsDot(outputType)
				outputImportAs := strings.Join(popFirst(strings.Split(outputPath, ".")), "_")

				if outputImportAs == "" {
					protoPath := strings.Split(dep.GetGrpcClient().Definition, ":")[0]
					protoParts := strings.Split(protoPath, "/")
					outputType = repo + "." + strings.Join(protoParts[:len(protoParts)-1], ".") + "." + outputName

					outputPath, outputName = parseImportPartsDot(outputType)
					outputImportAs = strings.Join(popFirst(strings.Split(outputPath, ".")), "_")
				}
				imports = append(imports,
					&ast.ImportSpec{
						Name: &ast.Ident{Name: outputImportAs},
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: wrap(shared.ImportPathForType(outputType, repo)),
						},
					},
					&ast.ImportSpec{
						Name: &ast.Ident{Name: inputImportAs},
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: wrap(shared.ImportPathForType(inputType, repo)),
						},
					},
					&ast.ImportSpec{
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: wrap("google.golang.org/grpc"),
						},
					},
					&ast.ImportSpec{
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: wrap("errors"),
						},
					},
				)
				if rpc.IsStreamInput || rpc.IsStreamOutput {
					fields = append(fields, &ast.Field{
						Names: []*ast.Ident{&ast.Ident{Name: fmt.Sprintf("%sCalls", rpc.Name)}},
						Type:  &ast.Ident{Name: "[]any"},
					})
					imports = append(imports, &ast.ImportSpec{
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: wrap("github.com/magicpantry/infra/shared/mocks"),
						},
					})

					fields = append(fields, &ast.Field{
						Names: []*ast.Ident{&ast.Ident{Name: fmt.Sprintf("Return%s", rpc.Name)}},
						Type: &ast.Ident{Name: fmt.Sprintf(
							"*mocks.ServerClient[*%s.%s, *%s.%s]",
							inputImportAs, inputName, outputImportAs, outputName)},
					})

					funcStatements := []ast.Stmt{
						&ast.AssignStmt{
							Tok: token.ASSIGN,
							Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("client.%sCalls", rpc.Name)}},
							Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("append(client.%sCalls, struct{}{})", rpc.Name)}},
						},
						&ast.IfStmt{
							Cond: &ast.Ident{Name: "client.ReturnErr"},
							Body: &ast.BlockStmt{
								List: []ast.Stmt{
									&ast.ReturnStmt{
										Results: []ast.Expr{
											&ast.Ident{Name: "nil"},
											&ast.Ident{Name: "errors.New(\"error\")"},
										},
									},
								},
							},
						},
						&ast.ReturnStmt{
							Results: []ast.Expr{
								&ast.Ident{Name: fmt.Sprintf("client.Return%s", rpc.Name)},
								&ast.Ident{Name: "nil"},
							},
						},
					}
					decls = append(decls, &ast.FuncDecl{
						Name: &ast.Ident{Name: rpc.Name},
						Recv: &ast.FieldList{
							List: []*ast.Field{
								&ast.Field{
									Names: []*ast.Ident{&ast.Ident{Name: "client"}},
									Type:  &ast.Ident{Name: fmt.Sprintf("*%sClient", dep.Name)},
								},
							},
						},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									&ast.Field{
										Names: []*ast.Ident{&ast.Ident{Name: "ctx"}},
										Type:  &ast.Ident{Name: "context.Context"},
									},
									&ast.Field{
										Names: []*ast.Ident{&ast.Ident{Name: "opts"}},
										Type:  &ast.Ident{Name: "...grpc.CallOption"},
									},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									&ast.Field{
										Type: &ast.Ident{
											Name: fmt.Sprintf("%s.%s_%sClient", importAs, name, rpc.Name),
										},
									},
									&ast.Field{
										Type: &ast.Ident{Name: "error"},
									},
								},
							},
						},
						Body: &ast.BlockStmt{
							List: funcStatements,
						},
					})
				} else {
					fields = append(fields, &ast.Field{
						Names: []*ast.Ident{&ast.Ident{Name: fmt.Sprintf("%sCalls", rpc.Name)}},
						Type:  &ast.Ident{Name: fmt.Sprintf("[]*%s.%s", inputImportAs, inputName)},
					})
					fields = append(fields, &ast.Field{
						Names: []*ast.Ident{&ast.Ident{Name: fmt.Sprintf("Return%s", rpc.Name)}},
						Type:  &ast.Ident{Name: fmt.Sprintf("*%s.%s", outputImportAs, outputName)},
					})

					funcStatements := []ast.Stmt{
						&ast.AssignStmt{
							Tok: token.ASSIGN,
							Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("client.%sCalls", rpc.Name)}},
							Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("append(client.%sCalls, arg)", rpc.Name)}},
						},
						&ast.IfStmt{
							Cond: &ast.Ident{Name: "client.ReturnErr"},
							Body: &ast.BlockStmt{
								List: []ast.Stmt{
									&ast.ReturnStmt{
										Results: []ast.Expr{
											&ast.Ident{Name: "nil"},
											&ast.Ident{Name: "errors.New(\"error\")"},
										},
									},
								},
							},
						},
						&ast.ReturnStmt{
							Results: []ast.Expr{
								&ast.Ident{Name: fmt.Sprintf("client.Return%s", rpc.Name)},
								&ast.Ident{Name: "nil"},
							},
						},
					}
					decls = append(decls, &ast.FuncDecl{
						Name: &ast.Ident{Name: rpc.Name},
						Recv: &ast.FieldList{
							List: []*ast.Field{
								&ast.Field{
									Names: []*ast.Ident{&ast.Ident{Name: "client"}},
									Type:  &ast.Ident{Name: fmt.Sprintf("*%sClient", dep.Name)},
								},
							},
						},
						Type: &ast.FuncType{
							Params: &ast.FieldList{
								List: []*ast.Field{
									&ast.Field{
										Names: []*ast.Ident{&ast.Ident{Name: "ctx"}},
										Type:  &ast.Ident{Name: "context.Context"},
									},
									&ast.Field{
										Names: []*ast.Ident{&ast.Ident{Name: "arg"}},
										Type:  &ast.Ident{Name: fmt.Sprintf("*%s.%s", inputImportAs, inputName)},
									},
									&ast.Field{
										Names: []*ast.Ident{&ast.Ident{Name: "opts"}},
										Type:  &ast.Ident{Name: "...grpc.CallOption"},
									},
								},
							},
							Results: &ast.FieldList{
								List: []*ast.Field{
									&ast.Field{
										Type: &ast.Ident{
											Name: fmt.Sprintf("*%s.%s", outputImportAs, outputName),
										},
									},
									&ast.Field{
										Type: &ast.Ident{Name: "error"},
									},
								},
							},
						},
						Body: &ast.BlockStmt{
							List: funcStatements,
						},
					})
				}
			}

			decls = append(decls,
				&ast.GenDecl{
					Tok: token.TYPE,
					Specs: []ast.Spec{
						&ast.TypeSpec{
							Name: &ast.Ident{Name: fmt.Sprintf("%sClient", dep.Name)},
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: fields,
								},
							},
						},
					},
				})
		}
		if dep.GetPool() != nil {
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: "&mocks.Pool{}"}},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
			})
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: dep.Name}},
				Type:  &ast.Ident{Name: "*mocks.Pool"},
			})
		}
		if dep.GetGrpcClientFactory() != nil {
			protoPath := strings.Split(dep.GetGrpcClientFactory().Definition, ":")[0]
			inputName := strings.Split(dep.GetGrpcClientFactory().Definition, ":")[1]
			protoParts := strings.Split(protoPath, "/")
			inputType := repo + "." + strings.Join(protoParts[:len(protoParts)-1], ".") + "." + inputName
			inputPath, inputName := parseImportPartsDot(inputType)
			inputImportAs := strings.Join(popFirst(strings.Split(inputPath, ".")), "_")

			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{
					&ast.Ident{Name: fmt.Sprintf("&mocks.ClientFactory[%s.%sClient]{}", inputImportAs, inputName)},
				},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{
					&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s.Func()", dep.Name)},
				},
			})
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: dep.Name}},
				Type:  &ast.Ident{Name: fmt.Sprintf("*mocks.ClientFactory[%s.%sClient]", inputImportAs, inputName)},
			})
		}
		if dep.GetBlockstore() != nil {
			imports = append(imports, &ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: wrap("github.com/magicpantry/infra/shared/mocks"),
				},
			})
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: dep.Name}},
				Type:  &ast.Ident{Name: "*mocks.BlockStore"},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: "&mocks.BlockStore{}"}},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
			})
		}
		if dep.GetDatabase() != nil {
			imports = append(imports, &ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: wrap("github.com/magicpantry/infra/shared/mocks"),
				},
			})
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: dep.Name}},
				Type:  &ast.Ident{Name: "*mocks.Database"},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: "&mocks.Database{}"}},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
			})
		}
		if dep.GetDocstore() != nil {
			imports = append(imports, &ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: wrap("github.com/magicpantry/infra/shared/mocks"),
				},
			})
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: dep.Name}},
				Type:  &ast.Ident{Name: "*mocks.DocStore"},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: "&mocks.DocStore{}"}},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
			})
		}
		if dep.GetElastic() != nil {
			sig := dep.GetElastic().Signature
			keyPath, keyName := parseImportParts(sig.KeyType)
			filterPath, filterName := parseImportParts(sig.FilterType)
			sortPath, sortName := parseImportParts(sig.SortType)

			keyImportAs := strings.Join(popLast(strings.Split(strings.Split(keyPath, ".")[0], "/")), "_")
			filterImportAs := strings.Join(popLast(strings.Split(strings.Split(filterPath, ".")[0], "/")), "_")
			sortImportAs := strings.Join(popLast(strings.Split(strings.Split(sortPath, ".")[0], "/")), "_")

			imports = append(imports, &ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: wrap("github.com/magicpantry/infra/shared/mocks"),
				},
			})
			imports = append(imports, &ast.ImportSpec{
				Name: &ast.Ident{Name: keyImportAs},
				Path: &ast.BasicLit{
					Kind: token.STRING,
					Value: wrap(fmt.Sprintf(
						"github.com/magicpantry/%s/gen/%s",
						repo,
						parseImportPath(keyPath))),
				},
			})
			imports = append(imports, &ast.ImportSpec{
				Name: &ast.Ident{Name: filterImportAs},
				Path: &ast.BasicLit{
					Kind: token.STRING,
					Value: wrap(fmt.Sprintf(
						"github.com/magicpantry/%s/gen/%s",
						repo,
						parseImportPath(filterPath))),
				},
			})
			imports = append(imports, &ast.ImportSpec{
				Name: &ast.Ident{Name: sortImportAs},
				Path: &ast.BasicLit{
					Kind: token.STRING,
					Value: wrap(fmt.Sprintf(
						"github.com/magicpantry/%s/gen/%s",
						repo,
						parseImportPath(sortPath))),
				},
			})

			mockTextSearch := fmt.Sprintf(
				"mocks.TextSearch[*%s.%s, *%s.%s, *%s.%s]",
				keyImportAs, keyName,
				filterImportAs, filterName,
				sortImportAs, sortName,
			)
			mockedFields = append(mockedFields, &ast.Field{
				Names: []*ast.Ident{&ast.Ident{Name: dep.Name}},
				Type:  &ast.Ident{Name: "*" + mockTextSearch},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("&%s{}", mockTextSearch)}},
			})
			statements = append(statements, &ast.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Manifest.Dependencies.%s", dep.Name)}},
				Rhs: []ast.Expr{&ast.Ident{Name: fmt.Sprintf("mockManifest.Mocked.%s", dep.Name)}},
			})
		}
	}

	decls = append([]ast.Decl{
		&ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: imports,
		},
		&ast.GenDecl{
			Tok: token.TYPE,
			Specs: []ast.Spec{
				&ast.TypeSpec{
					Name: &ast.Ident{Name: "Mocked"},
					Type: &ast.StructType{
						Fields: &ast.FieldList{
							List: mockedFields,
						},
					},
				},
			},
		},
	}, decls...)

	statements = append(statements, &ast.ReturnStmt{
		Results: []ast.Expr{&ast.Ident{Name: "mockManifest"}},
	})

	decls = append(decls,
		&ast.FuncDecl{
			Name: &ast.Ident{Name: "NewMockManifest"},
			Type: &ast.FuncType{
				Params: &ast.FieldList{
					List: []*ast.Field{
						&ast.Field{
							Names: []*ast.Ident{&ast.Ident{Name: "t"}},
							Type:  &ast.Ident{Name: "*testing.T"},
						},
					},
				},
				Results: &ast.FieldList{
					List: []*ast.Field{
						&ast.Field{
							Type: &ast.Ident{Name: "*MockManifest"},
						},
					},
				},
			},
			Body: &ast.BlockStmt{
				List: statements,
			},
		})

	f := &ast.File{Name: &ast.Ident{Name: "mockmanifest"}, Decls: decls}

	buf := new(bytes.Buffer)
	if err := format.Node(buf, fs, f); err != nil {
		log.Fatal(err)
	}

	return string(buf.Bytes())
}

func wrap(x string) string {
	return "\"" + x + "\""
}

func parseImportPartsDot(x string) (string, string) {
	parts := strings.Split(x, ".")
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func parseImportParts(x string) (string, string) {
	parts := strings.Split(x, ":")
	return parts[0], parts[1]
}

func popFirst(xs []string) []string {
	return xs[1:]
}

func popLast(xs []string) []string {
	return xs[:len(xs)-1]
}

func parseImportPath(x string) string {
	parts := strings.Split(x, "/")
	return strings.Join(parts[:len(parts)-1], "/")
}
