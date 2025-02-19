package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/magicpantry/infra/gen/proto"
	"github.com/magicpantry/infra/generate/dockerfile"
	"github.com/magicpantry/infra/generate/functionfile"
	"github.com/magicpantry/infra/generate/functionstub"
	"github.com/magicpantry/infra/generate/functionteststub"
	"github.com/magicpantry/infra/generate/grpcstub"
	"github.com/magicpantry/infra/generate/grpcteststub"
	"github.com/magicpantry/infra/generate/httpstub"
	"github.com/magicpantry/infra/generate/httpteststub"
	"github.com/magicpantry/infra/generate/jobstub"
	"github.com/magicpantry/infra/generate/jobteststub"
	"github.com/magicpantry/infra/generate/mainfile"
	"github.com/magicpantry/infra/generate/makefile"
	"github.com/magicpantry/infra/generate/manifestfile"
	"github.com/magicpantry/infra/generate/mockmanifestfile"
	"github.com/magicpantry/infra/generate/shared"
	"github.com/magicpantry/infra/generate/terraform"
	infra_shared "github.com/magicpantry/infra/shared"
	"github.com/magicpantry/infra/shared/gcp"
)

func main() {
	rootDir := infra_shared.RootDir()
	root := infra_shared.ReadRootAtPath(rootDir + "/root.textproto")
	repo := root.Repo
	for componentDir, manifest := range infra_shared.ReadManifest() {
		log.Printf("gen '%s/%s'\n", manifest.Component.Namespace, manifest.Component.Name)
		if manifest.Component.GetJob() != nil {
			genJob(componentDir, manifest)
		}
		if manifest.Component.GetModelServer() != nil {
			genModelServer(componentDir, manifest)
		}
		if manifest.Component.GetHttpServer() != nil {
			genHTTPServer(componentDir, manifest)
		}
		if manifest.Component.GetGrpcServer() != nil {
			genGRPCServer(componentDir, manifest)
		}
		if manifest.Component.GetFunction() != nil {
			genFunction(componentDir, manifest)
		}
		if manifest.Component.GetWebapp() != nil {
			genWebApp(componentDir, manifest)
		}
	}
	if err := os.MkdirAll(rootDir+"/terraform", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(rootDir+"/terraform/infra_gen.tf", []byte(terraform.Build(rootDir, infra_shared.ReadAllManifests(), repo)), 0644); err != nil {
		log.Fatal(err)
	}
}

func genWebApp(componentDir string, manifest *proto.Manifest) {
	paths := infra_shared.MakePaths(componentDir)
	root := infra_shared.ReadRootAtPath(paths.RootDir + "/root.textproto")
	repo := root.Repo

	if len(manifest.Config.Items) > 0 {
		if err := os.MkdirAll(paths.ComponentDir+"/src/app/gen/providers", os.ModePerm); err != nil {
			log.Fatal(err)
		}

		content := `import { Injectable } from '@angular/core';
import { ReplaySubject, Subscription } from 'rxjs';

export type Config = {
%s
}

@Injectable({
  providedIn: "root"
})
export class ConfigProvider {
  private readonly _config: Config;
  get config(): Config {
    return this._config;
  }

  constructor() {
    this._config = {
%s
    };
  }
}
`

		var configLines []string
		var initLines []string
		handleConfigItem := func(ci *proto.ConfigItem) {
			if ci.GetIntValue() != 0 {
				configLines = append(configLines, fmt.Sprintf("%s: number;", ci.Name))
				initLines = append(initLines, fmt.Sprintf("%s: %v,", ci.Name, ci.GetIntValue()))
			}
			if ci.GetDoubleValue() != 0 {
				configLines = append(configLines, fmt.Sprintf("%s: number;", ci.Name))
				initLines = append(initLines, fmt.Sprintf("%s: %v,", ci.Name, ci.GetDoubleValue()))
			}
			if ci.GetStringValue() != "" {
				configLines = append(configLines, fmt.Sprintf("%s: string;", ci.Name))
				initLines = append(initLines, fmt.Sprintf("%s: \"%v\",", ci.Name, ci.GetStringValue()))
			}
			if ci.GetListValue() != nil {
				configLines = append(configLines, fmt.Sprintf("%s: string[];", ci.Name))
				var listValues []string
				for _, v := range ci.GetListValue().Values {
					listValues = append(listValues, fmt.Sprintf("\"%s\"", v))
				}
				initLines = append(initLines, fmt.Sprintf("%s: [%v],", ci.Name, strings.Join(listValues, ", ")))
			}
		}

		rootConfigs := map[string]*proto.ConfigItem{}
		for _, item := range root.Config.Items {
			rootConfigs[item.Name] = item
		}
		for _, config := range manifest.Config.Items {
			if config.GetKeyValue() != nil {
				item, ok := rootConfigs[config.Name]
				if !ok {
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

		for i := range configLines {
			configLines[i] = fmt.Sprintf("  %s", configLines[i])
			initLines[i] = fmt.Sprintf("      %s", initLines[i])
		}

		content = fmt.Sprintf(content, strings.Join(configLines, "\n"), strings.Join(initLines, "\n"))

		if err := os.WriteFile(paths.ComponentDir+"/src/app/gen/providers/config.provider.ts", []byte(content), 0644); err != nil {
			log.Fatal(err)
		}
	}

	if err := os.MkdirAll(paths.ComponentDir+"/src/app/gen/services", os.ModePerm); err != nil {
		log.Fatal(err)
	}

	auth := manifest.Component.GetWebapp().Auth
	authContent := fmt.Sprintf(`import { Injectable } from '@angular/core';

import { ReplaySubject, firstValueFrom } from 'rxjs';
import { FirebaseApp, initializeApp } from 'firebase/app';
import {
  User, Auth, EmailAuthProvider,
  signOut, signInWithEmailAndPassword,
  getAuth, signInAnonymously, onAuthStateChanged,
  linkWithCredential, createUserWithEmailAndPassword,
  sendEmailVerification, deleteUser, applyActionCode,
  updateEmail, updatePassword,
} from 'firebase/auth';

const firebaseConfig = {
  apiKey: "%s",
  authDomain: "magicpantryio.firebaseapp.com",
  projectId: "magicpantryio",
  storageBucket: "magicpantryio.appspot.com",
  messagingSenderId: "%s",
  appId: "%s",
  measurementId: "%s"
};

@Injectable({
  providedIn: "root"
})
export class AuthService {
  private readonly userSubject = new ReplaySubject<User>(1);
  private readonly app: FirebaseApp;
  private readonly auth: Auth;
  public signedIn = false;

  constructor() {
    this.app = initializeApp(firebaseConfig);
    this.auth = getAuth();

    const userSubject = new ReplaySubject<User|null>(1);

    onAuthStateChanged(this.auth, (user) => {
      if (user) {
        this.signedIn = true;
        userSubject.next(user);
        this.userSubject.next(user);
      } else {
        this.signedIn = false;
        userSubject.next(null);
      }
    });

    userSubject.subscribe(user => {
      if (!user) {
        signInAnonymously(this.auth)
            .then(() => {
              this.signedIn = true;
            })
            .catch((error) => {
              this.signedIn = false;
            });
      }
    });
  }

  async updateEmail(newEmail: string): Promise<string> {
    const errorSubject = new ReplaySubject<string>(1);
    let localUser = await firstValueFrom(this.userSubject);

    await updateEmail(localUser, newEmail)
        .then((user) => {
          errorSubject.next("");
        })
        .catch((error) => {
          errorSubject.next(error.message);
        });

    return await firstValueFrom(errorSubject);
  }

  async updatePassword(newPassword: string): Promise<string> {
    const errorSubject = new ReplaySubject<string>(1);
    let localUser = await firstValueFrom(this.userSubject);

    await updatePassword(localUser, newPassword)
        .then((user) => {
          errorSubject.next("");
        })
        .catch((error) => {
          errorSubject.next(error.message);
        });

    return await firstValueFrom(errorSubject);
  }

  async deleteUser(): Promise<string> {
    const errorSubject = new ReplaySubject<string>(1);
    let localUser = await firstValueFrom(this.userSubject);

    await deleteUser(localUser)
        .then((user) => {
          errorSubject.next("");
        })
        .catch((error) => {
          errorSubject.next(error.message);
        });

    return await firstValueFrom(errorSubject);
  }

  async signOut(): Promise<void> {
    await this.auth.signOut();
  }

  async signIn(username: string, password: string): Promise<string> {
    const errorSubject = new ReplaySubject<string>(1);

    signInWithEmailAndPassword(this.auth, username, password)
        .then((user) => {
          errorSubject.next("");
        })
        .catch((error) => {
          errorSubject.next(error.message);
        });

    return await firstValueFrom(errorSubject);
  }

  async signUp(username: string, password: string, confirm: string): Promise<string> {
    const errorSubject1 = new ReplaySubject<string>(1);

    const credential = EmailAuthProvider.credential(username, password);
    let localUser = await firstValueFrom(this.userSubject);

    linkWithCredential(localUser, credential)
        .then((user) => {
          localUser = user.user;
          errorSubject1.next("");
        })
        .catch((error) => {
          errorSubject1.next(error.message);
        });

    const error = await firstValueFrom(errorSubject1);
    if (error !== '') {
      return error;
    }

    const errorSubject2 = new ReplaySubject<string>(1);
    sendEmailVerification(localUser)
        .then((user) => {
          errorSubject2.next("");
        })
        .catch((error) => {
          errorSubject2.next(error.message);
        });

    return await firstValueFrom(errorSubject2);
  }

  async sendAgain(): Promise<string> {
    let localUser = await firstValueFrom(this.userSubject);
    const errorSubject = new ReplaySubject<string>(1);
    sendEmailVerification(localUser)
        .then((user) => {
          errorSubject.next("");
        })
        .catch((error) => {
          errorSubject.next(error.message);
        });

    return await firstValueFrom(errorSubject);
  }

  async userInfo(): Promise<UserInfo> {
    const user = await firstValueFrom(this.userSubject);
    return {
      isSignedIn: !user.isAnonymous,
      isVerified: user.emailVerified,
      name: user.email ?? '',
    };
  }
  
  async verifyUser(oobCode: string): Promise<string> {
    const errorSubject = new ReplaySubject<string>(1);
    applyActionCode(this.auth, oobCode).then((resp) => {
      errorSubject.next('');
    }).catch((error) => {
      errorSubject.next(error.message);
    });
    return await firstValueFrom(errorSubject);
  }

  async isSignedIn(): Promise<boolean> {
    const user = await firstValueFrom(this.userSubject);
    return !user.isAnonymous;
  }

  async idToken(): Promise<string> {
    const user = await firstValueFrom(this.userSubject);
    return await user.getIdToken();
  }
}

export type UserInfo = {
  isSignedIn: boolean;
  isVerified: boolean;
  name: string;
}
`, auth.ApiKey, auth.MessagingSenderId, auth.AppId, auth.MeasurementId)
	if err := os.WriteFile(paths.ComponentDir+"/src/app/gen/services/auth.service.ts", []byte(authContent), 0644); err != nil {
		log.Fatal(err)
	}

	protoFiles := map[string]string{}
	if err := filepath.WalkDir(paths.RootDir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		parts := strings.Split(path, ".")
		if parts[len(parts)-1] == "proto" {
			bs, err := os.ReadFile(path)
			if err != nil {
				log.Fatal(err)
			}
			protoFiles[path] = string(bs)
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	contents := map[string][]messageField{}
	for _, item := range manifest.RuntimeDependencies.Items {
		if item.GetHttpClient() != nil {
			parts := strings.Split(item.GetHttpClient().Definition, "/")
			withoutLast := strings.Join(parts[:len(parts)-1], ".")
			rpcs := shared.FindServiceInfo(paths.RootDir, item.GetHttpClient().Definition)
			var imports []string
			var subjects []string
			var requestFuncs []string
			var subscribeFuncs []string

			for _, rpc := range rpcs {
				if strings.HasPrefix(rpc.Name, "Sizzle") {
					continue
				}

				findDefinitions(repo+"."+withoutLast+"."+rpc.InputType, protoFiles, contents)
				findDefinitions(repo+"."+withoutLast+"."+rpc.OutputType, protoFiles, contents)

				subjectName := fmt.Sprintf("%sSubject", strings.ToLower(rpc.Name[0:1])+rpc.Name[1:])

				subjects = append(subjects, fmt.Sprintf(`  private readonly %s = new ReplaySubject<%s>();`, subjectName, rpc.OutputType))
				imports = append(
					imports,
					fmt.Sprintf("import { %s, %s, transformToJSON%s, transformFromJSON%s } from '../messages/%s';",
						rpc.InputType, rpc.OutputType,
						rpc.InputType, rpc.OutputType,
						strings.ReplaceAll(withoutLast, ".", "/")))
				requestFuncs = append(requestFuncs, fmt.Sprintf(`  async request%s(v: %s) {
    const idToken = await this.authService.idToken();
    const requestOptions = {
      headers: {
        'Authorization': 'Bearer ' + idToken,
      },
    };
    this.http.post(this.url, transformToJSON%s(v), requestOptions).subscribe(
      data => this.%s.next(transformFromJSON%s(data)),
      error => this.errorSubject.next(error));
  }`, rpc.Name, rpc.InputType, rpc.InputType, subjectName, rpc.OutputType))
				subscribeFuncs = append(subscribeFuncs, fmt.Sprintf(`  subscribeTo%s(fn: (v: %s) => void): Subscription {
    return this.%s.subscribe(fn);
  }`, rpc.Name, rpc.OutputType, subjectName))
			}

			if err := os.MkdirAll(paths.ComponentDir+"/src/app/gen/services", os.ModePerm); err != nil {
				log.Fatal(err)
			}

			content := `import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { ReplaySubject, Subscription } from 'rxjs';
import { AuthService } from './auth.service';

%s
import { convertToURLSearchParams } from '../../utils/requests';

@Injectable({
  providedIn: "root"
})
export class APIService {

  private readonly url = '%s';
%s
  private readonly errorSubject = new ReplaySubject<string>();

  constructor(private http: HttpClient, private authService: AuthService) { }

%s

%s

  subscribeToErrors(fn: (v: string) => void): Subscription {
    return this.errorSubject.subscribe(fn);
  }
}
`

			content = fmt.Sprintf(
				content,
				strings.Join(imports, "\n"),
				item.GetHttpClient().Url,
				strings.Join(subjects, "\n"),
				strings.Join(requestFuncs, "\n"),
				strings.Join(subscribeFuncs, "\n"),
			)

			if err := os.WriteFile(paths.ComponentDir+"/src/app/gen/services/"+strings.ToLower(item.Name)+".service.ts", []byte(content), 0644); err != nil {
				log.Fatal(err)
			}
		}
	}

	byPackage := groupByPackage(contents, repo)

	for packageName, typeFieldsPair := range byPackage {
		genTypesFile(paths, packageName, typeFieldsPair)
	}
}

type messageField struct {
	Type    string
	Name    string
	IsArray bool
}

type message struct {
	Type   string
	Fields []messageField
}

func genTypesFile(paths infra_shared.Paths, packageName string, typeFieldsPairs map[string][]messageField) {
	parts := strings.Split(packageName, ".")
	pathName := strings.Join(parts[1:], ".")
	fileName := paths.ComponentDir + "/src/app/gen/messages/" + strings.ReplaceAll(pathName, ".", "/") + ".ts"

	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		log.Fatal(err)
	}

	var messages []message
	for typeName, rawFields := range typeFieldsPairs {
		var fields []messageField
		for _, rawField := range rawFields {
			fields = append(fields, rawField)
		}
		messages = append(messages, message{
			Type:   typeName,
			Fields: fields,
		})
	}
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Type < messages[j].Type
	})

	addStringValue := false
	addUInt32Value := false
	addInt32Value := false
	addFloatValue := false
	addDuration := false

	var imports []string
	var lines []string
	enums := map[string]string{}
	for _, message := range messages {
		if isEnum(message.Fields) {
			enums[message.Type] = message.Fields[0].Type
			lines = append(lines, fmt.Sprintf("export enum %s {", message.Type))
			for _, field := range message.Fields {
				lines = append(lines, fmt.Sprintf("\t%s,", convertSnakeCaseToTitleCase(field.Type)))
			}
			lines = append(lines, "}")
			lines = append(lines, fmt.Sprintf("export function transformToString%s(v: %s): any {", message.Type, message.Type))
			lines = append(lines, "\tswitch (v) {")
			for _, field := range message.Fields {
				lines = append(lines, fmt.Sprintf("\t\tcase %s.%s:", message.Type, convertSnakeCaseToTitleCase(field.Type)))
				lines = append(lines, fmt.Sprintf("\t\t\treturn \"%s\"", field.Type))
			}
			lines = append(lines, "\t\tdefault:")
			lines = append(lines, fmt.Sprintf("\t\t\treturn \"%s\"", message.Fields[0].Type))
			lines = append(lines, "\t}")
			lines = append(lines, "}")

			lines = append(lines, fmt.Sprintf("export function transformFromString%s(v: any): %s {", message.Type, message.Type))
			lines = append(lines, "\tswitch (v) {")
			for _, field := range message.Fields {
				lines = append(lines, fmt.Sprintf("\t\tcase \"%s\":", field.Type))
				lines = append(lines, fmt.Sprintf("\t\t\treturn %s.%s", message.Type, convertSnakeCaseToTitleCase(field.Type)))
			}
			lines = append(lines, "\t\tdefault:")
			lines = append(lines, fmt.Sprintf("\t\t\treturn %s.%s", message.Type, convertSnakeCaseToTitleCase(message.Fields[0].Type)))
			lines = append(lines, "\t}")
			lines = append(lines, "}")
		} else {
			lines = append(lines, fmt.Sprintf("export type %s = {", message.Type))
			for _, field := range message.Fields {
				parts := strings.Split(field.Type, ".")
				otherPackageName := strings.Join(parts[:len(parts)-1], ".")
				typeName := parts[len(parts)-1]
				fieldName := convertSnakeCaseToCamelCase(field.Name)
				var prefixParts []string
				for i := 0; i < len(strings.Split(pathName, "."))-1; i++ {
					prefixParts = append(prefixParts, "..")
				}
				if otherPackageName != "" && packageName != otherPackageName {
					if otherPackageName == "google.protobuf" {
						if typeName == "StringValue" {
							addStringValue = true
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/stringvalue"
							imports = append(imports, fmt.Sprintf("import { %s } from '%s';", typeName, externalPath))
						}
						if typeName == "UInt32Value" {
							addUInt32Value = true
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/uint32value"
							imports = append(imports, fmt.Sprintf("import { %s } from '%s';", typeName, externalPath))
						}
						if typeName == "Int32Value" {
							addInt32Value = true
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/int32value"
							imports = append(imports, fmt.Sprintf("import { %s } from '%s';", typeName, externalPath))
						}
						if typeName == "FloatValue" {
							addFloatValue = true
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/floatvalue"
							imports = append(imports, fmt.Sprintf("import { %s } from '%s';", typeName, externalPath))
						}
						if typeName == "Timestamp" {
							typeName = "Date"
						}
						if typeName == "Duration" {
							addDuration = true
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/duration"
							imports = append(imports, fmt.Sprintf("import { %s } from '%s';", typeName, externalPath))
						}
					} else {
						otherParts := strings.Split(otherPackageName, ".")
						otherFileName := strings.Join(otherParts[1:], "/")

						otherFileName = strings.Join(prefixParts, "/") + "/" + otherFileName
						imports = append(imports, fmt.Sprintf("import { %s } from '%s';", typeName, otherFileName))
					}
				}
				if isAllLower(typeName) {
					if !field.IsArray {
						lines = append(
							lines,
							fmt.Sprintf("\t%s: %s;", fieldName, replaceBuiltIn(typeName)))
					} else {
						lines = append(
							lines,
							fmt.Sprintf("\t%s: %s[];", fieldName, replaceBuiltIn(typeName)))
					}
				} else {
					if !field.IsArray {
						lines = append(
							lines,
							fmt.Sprintf("\t%s?: %s;", fieldName, typeName))
					} else {
						lines = append(
							lines,
							fmt.Sprintf("\t%s?: %s[];", fieldName, typeName))
					}
				}
			}
			lines = append(lines, "}")
			if message.Type != "Duration" {
				lines = append(lines, fmt.Sprintf("export function transformFromJSON%s(v: any): %s {", message.Type, message.Type))
				lines = append(lines, "\treturn {")
				for _, field := range message.Fields {
					parts := strings.Split(field.Type, ".")
					typeName := parts[len(parts)-1]
					var prefixParts []string
					for i := 0; i < len(strings.Split(pathName, "."))-1; i++ {
						prefixParts = append(prefixParts, "..")
					}

					if strings.Contains(field.Type, ".") {
						otherParts := strings.Split(field.Type, ".")
						otherPackageName := strings.Join(otherParts[:len(otherParts)-1], ".")
						if otherPackageName != "" && packageName != otherPackageName && otherPackageName != "google.protobuf" {
							otherFileName := strings.Join(otherParts[1:len(otherParts)-1], "/")
							otherFileName = strings.Join(prefixParts, "/") + "/" + otherFileName
							imports = append(imports, fmt.Sprintf("import { transformFromJSON%s } from '%s';", typeName, otherFileName))
						}
					}

					if _, ok := enums[typeName]; ok {
						lines = append(
							lines,
							fmt.Sprintf("\t\t%s: v.%s != null ? transformFromString%s(v.%s) : %s.%s,",
								convertSnakeCaseToCamelCase(field.Name),
								convertSnakeCaseToCamelCase(field.Name),
								typeName, convertSnakeCaseToCamelCase(field.Name),
								typeName, convertSnakeCaseToTitleCase(enums[typeName])))
					} else {
						if typeName == "Duration" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/duration"
							imports = append(imports, fmt.Sprintf("import { transformFromStringDuration } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformFromStringDuration(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "StringValue" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/stringvalue"
							imports = append(imports, fmt.Sprintf("import { transformFromJSONStringValue } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformFromJSONStringValue(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "UInt32Value" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/uint32value"
							imports = append(imports, fmt.Sprintf("import { transformFromJSONUInt32Value } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformFromJSONUInt32Value(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "Int32Value" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/int32value"
							imports = append(imports, fmt.Sprintf("import { transformFromJSONInt32Value } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformFromJSONInt32Value(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "FloatValue" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/floatvalue"
							imports = append(imports, fmt.Sprintf("import { transformFromJSONFloatValue } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformFromJSONFloatValue(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "Timestamp" {
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? new Date(Date.parse(v.%s)) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if isBuiltInOrEnum(typeName) {
							var primitive string
							if typeName == "bool" {
								primitive = "false"
							}
							if typeName == "string" {
								primitive = "\"\""
							}
							if typeName == "bytes" {
								primitive = "[]"
							}
							if typeName == "uint64" {
								primitive = "0"
							}
							if typeName == "uint32" {
								primitive = "0"
							}
							if typeName == "int64" {
								primitive = "0"
							}
							if typeName == "int32" {
								primitive = "0"
							}
							if typeName == "float" {
								primitive = "0.0"
							}
							if typeName == "double" {
								primitive = "0.0"
							}
							if field.IsArray {
								primitive = "[]"
							}
							if primitive != "" {
								lines = append(
									lines,
									fmt.Sprintf("\t\t%s: v.%s ?? %s,",
										convertSnakeCaseToCamelCase(field.Name),
										convertSnakeCaseToCamelCase(field.Name),
										primitive))
							}
						} else {
							if field.IsArray {
								lines = append(
									lines,
									fmt.Sprintf("\t\t%s: v.%s != null ? v.%s.map(transformFromJSON%s) : undefined,",
										convertSnakeCaseToCamelCase(field.Name),
										convertSnakeCaseToCamelCase(field.Name),
										convertSnakeCaseToCamelCase(field.Name),
										typeName))
							} else {
								lines = append(
									lines,
									fmt.Sprintf("\t\t%s: v.%s != null ? transformFromJSON%s(v.%s) : undefined,",
										convertSnakeCaseToCamelCase(field.Name),
										convertSnakeCaseToCamelCase(field.Name),
										typeName, convertSnakeCaseToCamelCase(field.Name)))
							}
						}
					}
				}
				lines = append(lines, "\t};")
				lines = append(lines, "}")

				lines = append(lines, fmt.Sprintf("export function transformToJSON%s(v: %s): any {", message.Type, message.Type))
				lines = append(lines, "\treturn {")
				for _, field := range message.Fields {
					parts := strings.Split(field.Type, ".")
					typeName := parts[len(parts)-1]
					var prefixParts []string
					for i := 0; i < len(strings.Split(pathName, "."))-1; i++ {
						prefixParts = append(prefixParts, "..")
					}

					if strings.Contains(field.Type, ".") {
						otherParts := strings.Split(field.Type, ".")
						otherPackageName := strings.Join(otherParts[:len(otherParts)-1], ".")
						if otherPackageName != "" && packageName != otherPackageName && otherPackageName != "google.protobuf" {
							otherFileName := strings.Join(otherParts[1:len(otherParts)-1], "/")
							otherFileName = strings.Join(prefixParts, "/") + "/" + otherFileName
							imports = append(imports, fmt.Sprintf("import { transformToJSON%s } from '%s';", typeName, otherFileName))
						}
					}

					if _, ok := enums[typeName]; ok {
						lines = append(
							lines,
							fmt.Sprintf("\t\t%s: v.%s != null ? transformToString%s(v.%s) : %s.%s,",
								convertSnakeCaseToCamelCase(field.Name),
								convertSnakeCaseToCamelCase(field.Name),
								typeName, convertSnakeCaseToCamelCase(field.Name),
								typeName, convertSnakeCaseToTitleCase(enums[typeName])))
					} else {
						if typeName == "Duration" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/duration"
							imports = append(imports, fmt.Sprintf("import { transformToStringDuration } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformToStringDuration(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "StringValue" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/stringvalue"
							imports = append(imports, fmt.Sprintf("import { transformToJSONStringValue } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformToJSONStringValue(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "UInt32Value" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/uint32value"
							imports = append(imports, fmt.Sprintf("import { transformToJSONUInt32Value } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformToJSONUInt32Value(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "Int32Value" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/int32value"
							imports = append(imports, fmt.Sprintf("import { transformToJSONInt32Value } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformToJSONInt32Value(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "FloatValue" {
							externalPath := strings.Join(prefixParts, "/") + "/google/protobuf/floatvalue"
							imports = append(imports, fmt.Sprintf("import { transformToJSONFloatValue } from '%s';", externalPath))
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? transformToJSONFloatValue(v.%s) : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if typeName == "Timestamp" {
							lines = append(
								lines,
								fmt.Sprintf("\t\t%s: v.%s != null ? {seconds: v.%s.getTime() / 1000} : undefined,",
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name),
									convertSnakeCaseToCamelCase(field.Name)))
						} else if isBuiltInOrEnum(typeName) {
							var primitive string
							if typeName == "bool" {
								primitive = "false"
							}
							if typeName == "string" {
								primitive = "\"\""
							}
							if typeName == "bytes" {
								primitive = "[]"
							}
							if typeName == "uint64" {
								primitive = "0"
							}
							if typeName == "uint32" {
								primitive = "0"
							}
							if typeName == "int64" {
								primitive = "0"
							}
							if typeName == "int32" {
								primitive = "0"
							}
							if typeName == "float" {
								primitive = "0.0"
							}
							if field.IsArray {
								primitive = "[]"
							}
							if primitive != "" {
								lines = append(
									lines,
									fmt.Sprintf("\t\t%s: v.%s ?? %s,",
										convertSnakeCaseToCamelCase(field.Name),
										convertSnakeCaseToCamelCase(field.Name),
										primitive))
							}
						} else {
							if field.IsArray {
								lines = append(
									lines,
									fmt.Sprintf("\t\t%s: v.%s != null ? v.%s.map(transformToJSON%s) : undefined,",
										convertSnakeCaseToCamelCase(field.Name),
										convertSnakeCaseToCamelCase(field.Name),
										convertSnakeCaseToCamelCase(field.Name),
										typeName))
							} else {
								lines = append(
									lines,
									fmt.Sprintf("\t\t%s: v.%s != null ? transformToJSON%s(v.%s) : undefined,",
										convertSnakeCaseToCamelCase(field.Name),
										convertSnakeCaseToCamelCase(field.Name),
										typeName, convertSnakeCaseToCamelCase(field.Name)))
							}
						}
					}
				}
				lines = append(lines, "\t};")
				lines = append(lines, "}")
			}
		}
	}

	if addStringValue {
		genValue(paths, "String", "string")
	}
	if addUInt32Value {
		genValue(paths, "UInt32", "number")
	}
	if addInt32Value {
		genValue(paths, "Int32", "number")
	}
	if addFloatValue {
		genValue(paths, "Float", "number")
	}
	if addDuration {
		genDuration(paths)
	}
	fileName = paths.ComponentDir + "/src/app/gen/messages/" + strings.ReplaceAll(pathName, ".", "/") + ".ts"

	if strings.ReplaceAll(pathName, ".", "/")+".ts" == "protobuf.ts" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		log.Fatal(err)
	}

	content := ""
	if len(imports) > 0 {
		importSet := map[string]any{}
		for _, importValue := range imports {
			importSet[importValue] = struct{}{}
		}
		var filtered []string
		for x := range importSet {
			filtered = append(filtered, x)
		}
		sort.Strings(filtered)
		content = strings.Join(filtered, "\n") + "\n"
	}

	if err := os.WriteFile(fileName, []byte(content+strings.Join(lines, "\n")), 0644); err != nil {
		log.Fatal(err)
	}
}

func genValue(paths infra_shared.Paths, prefix, typeName string) {
	fileName := paths.ComponentDir + fmt.Sprintf("/src/app/gen/messages/google/protobuf/%svalue.ts", strings.ToLower(prefix))
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(fileName, []byte(fmt.Sprintf(`export type %sValue = {
    value: %s;
}

export function transformFromJSON%sValue(v: any): %sValue {
  return {
    value: v ?? undefined,
  };
}

export function transformToJSON%sValue(v: %sValue): any {
  return {
    value: v.value ?? undefined,
  };
}`, prefix, typeName, prefix, prefix, prefix, prefix)), 0644); err != nil {
		log.Fatal(err)
	}
}

func genDuration(paths infra_shared.Paths) {
	fileName := paths.ComponentDir + "/src/app/gen/messages/google/protobuf/duration.ts"
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(fileName, []byte(`export type Duration = {
    seconds: number;
    nanos: number;
}

export function transformFromStringDuration(v: string): Duration {
  return {
    seconds: Number(v.substr(0, v.length - 1)),
    nanos: 0,
  };
}

export function transformToStringDuration(v: Duration): string {
  return v.seconds + 's';
}`), 0644); err != nil {
		log.Fatal(err)
	}
}

func replaceBuiltIn(x string) string {
	if x == "bytes" {
		return "Uint8Array"
	}
	if x == "uint64" {
		return "number"
	}
	if x == "uint32" {
		return "number"
	}
	if x == "double" {
		return "number"
	}
	if x == "bool" {
		return "boolean"
	}
	return x
}

func convertSnakeCaseToCamelCase(x string) string {
	var transformed []string
	for _, part := range strings.Split(x, "_") {
		transformed = append(transformed, strings.ToUpper(part[0:1])+strings.ToLower(part[1:]))
	}
	combined := strings.Join(transformed, "")
	return strings.ToLower(combined[0:1]) + combined[1:]
}

func convertSnakeCaseToTitleCase(x string) string {
	var transformed []string
	for _, part := range strings.Split(x, "_") {
		transformed = append(transformed, part[0:1]+strings.ToLower(part[1:]))
	}
	return strings.Join(transformed, "")
}

func isEnum(mfs []messageField) bool {
	if len(mfs) == 0 {
		return false
	}
	for _, mf := range mfs {
		if mf.Name != "" {
			return false
		}
	}
	return true
}

func groupByPackage(types map[string][]messageField, repo string) map[string]map[string][]messageField {
	byPackage := map[string]map[string][]messageField{}
	for key, value := range types {
		if len(value) == 0 && !strings.HasPrefix(key, repo) {
			continue
		}
		parts := strings.Split(key, ".")
		packageName := strings.Join(parts[:len(parts)-1], ".")
		if _, ok := byPackage[packageName]; !ok {
			byPackage[packageName] = map[string][]messageField{}
		}
		byPackage[packageName][parts[len(parts)-1]] = value
	}
	return byPackage
}

func findDefinitions(t string, pfs map[string]string, types map[string][]messageField) {
	contents := findContents(t, pfs)
	types[t] = contents
	for _, content := range contents {
		findDefinitions(content.Type, pfs, types)
	}
}

func findContents(t string, pfs map[string]string) []messageField {
	parts := strings.Split(t, ".")
	packageName := strings.Join(parts[:len(parts)-1], ".")
	typeName := parts[len(parts)-1]

	var fields []messageField
	for _, pf := range pfs {
		if !strings.Contains(pf, fmt.Sprintf("package %s;", packageName)) {
			continue
		}

		lines := strings.Split(pf, "\n")
		start := -1
		end := -1
		for i, line := range lines {
			found := false
			if strings.HasPrefix(strings.TrimSpace(line), fmt.Sprintf("message %s {", typeName)) {
				found = true
			}
			if strings.HasPrefix(strings.TrimSpace(line), fmt.Sprintf("enum %s {", typeName)) {
				found = true
			}

			if found && start == -1 {
				start = i
				if strings.HasSuffix(line, "}") {
					end = i
				}
			}
			if start != -1 && end == -1 && line == "}" {
				end = i
			}
		}
		if start == -1 || end == -1 {
			continue
		}

		var filtered []string
		skipCount := 0
		for _, line := range lines[start : end+1] {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "oneof ") {
				skipCount++
				continue
			}
			if strings.HasPrefix(line, "}") && skipCount > 0 {
				skipCount--
				continue
			}
			if line == "" {
				continue
			}
			filtered = append(filtered, line)
		}
		lines = filtered

		combined := strings.Join(lines, "\n")
		body := strings.TrimSpace(combined[strings.Index(combined, "{")+1 : strings.LastIndex(combined, "}")])

		for _, line := range strings.Split(body, ";") {
			line = strings.TrimSpace(line)
			if len(line) == 0 || strings.HasPrefix(line, "}") {
				continue
			}
			if strings.HasPrefix(line, "//") {
				split := strings.Split(line, "\n")
				line = split[len(split)-1]
			}

			split := strings.Fields(line)
			var name, fieldName string
			isArray := false
			if split[0] == "repeated" {
				isArray = true
				name = split[1]
				fieldName = split[2]
			} else {
				name = split[0]
				fieldName = split[1]
			}
			formattedName := name
			if !isBuiltInOrEnum(name) && strings.Index(name, ".") == -1 {
				fields = append(fields, messageField{
					Type:    packageName + "." + formattedName,
					Name:    fieldName,
					IsArray: isArray,
				})
			} else {
				if isAllUpper(name) {
					fields = append(fields, messageField{
						Type:    formattedName,
						Name:    "",
						IsArray: isArray,
					})
				} else {
					fields = append(fields, messageField{
						Type:    formattedName,
						Name:    fieldName,
						IsArray: isArray,
					})
				}
			}
		}

		break
	}

	return fields
}

func isBuiltInOrEnum(x string) bool {
	return isAllLower(x) || isAllUpper(x)
}

func isAllUpper(x string) bool {
	for _, r := range x {
		if unicode.IsLower(r) && unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func isAllLower(x string) bool {
	for _, r := range x {
		if unicode.IsUpper(r) && unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func genFunction(componentDir string, manifest *proto.Manifest) {
	protos := filter(manifest.BuildDependencies.Items, func(x string) bool {
		return strings.HasSuffix(x, ".proto")
	})

	paths := infra_shared.MakePaths(componentDir)
	root := infra_shared.ReadRootAtPath(paths.RootDir + "/root.textproto")
	repo := root.Repo

	if len(protos) > 0 {
		if err := os.MkdirAll(paths.WorkspaceDir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(paths.WorkspaceDir+"/Makefile", []byte(makefile.Build(manifest.Component, paths, paths.PrefixFromGenToRoot, protos)), 0644); err != nil {
			log.Fatal(err)
		}

		infra_shared.Run("cd " + paths.WorkspaceDir + " && make")
	}

	if err := os.MkdirAll(paths.GenDir+"/manifest", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/mockmanifest", os.ModePerm); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(paths.GenDir+"/manifest/manifest.go", []byte(manifestfile.Build(paths, manifest, repo)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/mockmanifest/manifest.go", []byte(mockmanifestfile.Build(paths, manifest, nil, repo)), 0644); err != nil {
		log.Fatal(err)
	}

	functionManifest := manifest.Component.GetFunction()

	path := paths.ComponentDir + "/handler/handle.go"
	if _, err := os.Stat(path); err != nil {
		if err := os.MkdirAll(paths.ComponentDir+"/handler", os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(functionstub.Build(paths, functionManifest, repo)), 0644); err != nil {
			log.Fatal(err)
		}
	}
	testPath := paths.ComponentDir + "/handler/handle_test.go"
	if _, err := os.Stat(testPath); err != nil {
		if err := os.MkdirAll(paths.ComponentDir+"/handler", os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(testPath, []byte(functionteststub.Build(paths, manifest, repo)), 0644); err != nil {
			log.Fatal(err)
		}
	}

	if err := os.WriteFile(paths.RootDir+"/functions.go", []byte(functionfile.Build(paths, manifest, repo)), 0644); err != nil {
		log.Fatal(err)
	}

	infra_shared.Run("cd " + paths.GenDir + "/manifest" + " && go fmt")
	infra_shared.Run("cd " + paths.GenDir + "/mockmanifest" + " && go fmt")
	infra_shared.Run("cd " + paths.ComponentDir + " && go fmt ./...")
	infra_shared.Run("cd " + paths.RootDir + " && go fmt")
}

func genJob(componentDir string, manifest *proto.Manifest) {
	paths := infra_shared.MakePaths(componentDir)
	root := infra_shared.ReadRootAtPath(paths.RootDir + "/root.textproto")
	repo := root.Repo

	protos := filter(manifest.BuildDependencies.Items, func(x string) bool {
		return strings.HasSuffix(x, ".proto")
	})

	if len(protos) > 0 {
		if err := os.MkdirAll(paths.WorkspaceDir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(paths.WorkspaceDir+"/Makefile", []byte(makefile.Build(manifest.Component, paths, paths.PrefixFromGenToRoot, protos)), 0644); err != nil {
			log.Fatal(err)
		}

		infra_shared.Run("cd " + paths.WorkspaceDir + " && make")
	}

	if err := os.MkdirAll(paths.GenDir+"/cmd", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/manifest", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/mockmanifest", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/infra", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/infra/Dockerfile", []byte(dockerfile.Build(paths, manifest)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/cmd/main.go", []byte(mainfile.Build(paths, manifest, nil, repo)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/manifest/manifest.go", []byte(manifestfile.Build(paths, manifest, repo)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/mockmanifest/manifest.go", []byte(mockmanifestfile.Build(paths, manifest, nil,
		repo)), 0644); err != nil {
		log.Fatal(err)
	}

	func() {
		path := paths.ComponentDir + "/handlers/handler.go"
		if _, err := os.Stat(path); err == nil {
			return
		}
		if err := os.MkdirAll(paths.ComponentDir+"/handlers", os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(jobstub.Build(paths, repo)), 0644); err != nil {
			log.Fatal(err)
		}
	}()
	func() {
		path := paths.ComponentDir + "/handlers/handler_test.go"
		if _, err := os.Stat(path); err == nil {
			return
		}
		if err := os.MkdirAll(paths.ComponentDir+"/handlers", os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(jobteststub.Build(paths, repo)), 0644); err != nil {
			log.Fatal(err)
		}
	}()

	infra_shared.Run("cd " + paths.GenDir + "/manifest" + " && go fmt")
	infra_shared.Run("cd " + paths.GenDir + "/mockmanifest" + " && go fmt")
	infra_shared.Run("cd " + paths.GenDir + "/cmd" + " && go fmt")
}

func genModelServer(componentDir string, manifest *proto.Manifest) {
	paths := infra_shared.MakePaths(componentDir)

	protos := []string{"infra/proto/model.proto"}

	if err := os.MkdirAll(paths.WorkspaceDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.WorkspaceDir+"/Makefile", []byte(makefile.Build(manifest.Component, paths, paths.PrefixFromGenToRoot, protos)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/cmd", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/infra", os.ModePerm); err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(paths.InfraDir + "/generate/tmpls/model_server.py")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	destination, err := os.Create(paths.GenDir + "/cmd/__main__.py")
	if err != nil {
		log.Fatal(err)
	}
	defer destination.Close()

	if _, err := io.Copy(destination, f); err != nil {
		log.Fatal(err)
	}

	storage := gcp.StorageClient(context.Background())
	jsonMap := map[string][]map[string]string{
		"models": []map[string]string{},
	}
	for _, model := range manifest.Component.GetModelServer().Models {
		parts := strings.Split(model.Path, "/")
		name := parts[len(parts)-1]
		genPath := paths.GenDir + "/cmd/" + name
		path := strings.TrimPrefix(paths.GenDir+"/cmd/"+name, paths.RootDir+"/")

		bucketAndName := strings.TrimPrefix(model.Path, "gs://")
		bucket := strings.Split(bucketAndName, "/")[0]
		storageName := strings.Join(strings.Split(bucketAndName, "/")[1:], "/")

		if _, err := os.Stat(genPath); errors.Is(err, os.ErrNotExist) {
			func() {
				rd, err := storage.Bucket(bucket).Object(storageName).NewReader(context.Background())
				if err != nil {
					log.Fatal(err)
				}
				defer rd.Close()

				modelDest, err := os.Create(genPath)
				if err != nil {
					log.Fatal(err)
				}
				defer modelDest.Close()

				if _, err := io.Copy(modelDest, rd); err != nil {
					log.Fatal(err)
				}
			}()
		}

		jsonMap["models"] = append(jsonMap["models"], map[string]string{
			"name": model.Id,
			"path": path,
		})
	}

	if manifest.Component.GetModelServer() != nil && manifest.Component.GetModelServer().Target.GetCluster() != nil {
		cluster := manifest.Component.GetModelServer().Target.GetCluster()

		deployment := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      serviceAccountName: %s
      nodeSelector:
        iam.gke.io/gke-metadata-server-enabled: "true"
      containers:
      - name: %s
        image: gcr.io/magicpantryio/%s-%s:latest
        ports:
        - containerPort: %d
        imagePullPolicy: Always
        env:
          - name: PORT
            value: "%d"`, cluster.Deployment, cluster.Replicas, cluster.Label, cluster.Label, cluster.ServiceAccount, cluster.Container, manifest.Component.Namespace, manifest.Component.Name, cluster.Port, cluster.Port)

		service := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  annotations:
    networking.gke.io/load-balancer-type: "%s"
spec:
  type: LoadBalancer
  externalTrafficPolicy: Cluster
  selector:
    app: %s
  ports:
  - port: %d
    targetPort: %d`, cluster.ServiceName, "Internal", cluster.Label, cluster.Port, cluster.Port)

		if err := os.WriteFile(paths.GenDir+"/infra/deployment.yaml", []byte(deployment), 0644); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(paths.GenDir+"/infra/service.yaml", []byte(service), 0644); err != nil {
			log.Fatal(err)
		}
	}

	bs, err := json.MarshalIndent(jsonMap, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	if err := ioutil.WriteFile(paths.GenDir+"/cmd/config.json", bs, 0644); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(paths.GenDir+"/infra/Dockerfile", []byte(dockerfile.Build(paths, manifest)), 0644); err != nil {
		log.Fatal(err)
	}

	infra_shared.Run("cd " + paths.WorkspaceDir + " && make")
}

func genHTTPServer(componentDir string, manifest *proto.Manifest) {
	paths := infra_shared.MakePaths(componentDir)
	root := infra_shared.ReadRootAtPath(paths.RootDir + "/root.textproto")
	repo := root.Repo
	httpServerManifest := manifest.Component.GetHttpServer()

	if err := os.MkdirAll(paths.GenDir+"/cmd", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/manifest", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/mockmanifest", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/infra", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/infra/Dockerfile", []byte(dockerfile.Build(paths, manifest)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/cmd/main.go", []byte(mainfile.Build(paths, manifest, nil, repo)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/manifest/manifest.go", []byte(manifestfile.Build(paths, manifest, repo)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/mockmanifest/manifest.go", []byte(mockmanifestfile.Build(paths, manifest, nil,
		repo)), 0644); err != nil {
		log.Fatal(err)
	}

	func() {
		path := paths.ComponentDir + "/handlers/handler.go"
		if _, err := os.Stat(path); err == nil {
			return
		}
		if err := os.MkdirAll(paths.ComponentDir+"/handlers", os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(httpstub.Build(paths, httpServerManifest, repo)), 0644); err != nil {
			log.Fatal(err)
		}
	}()
	func() {
		path := paths.ComponentDir + "/handlers/handler_test.go"
		if _, err := os.Stat(path); err == nil {
			return
		}
		if err := os.MkdirAll(paths.ComponentDir+"/handlers", os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(httpteststub.Build(paths, manifest, repo)), 0644); err != nil {
			log.Fatal(err)
		}
	}()

	infra_shared.Run("cd " + paths.GenDir + "/manifest" + " && go fmt")
	infra_shared.Run("cd " + paths.GenDir + "/mockmanifest" + " && go fmt")
	infra_shared.Run("cd " + paths.GenDir + "/cmd" + " && go fmt")
}

func genGRPCServer(componentDir string, manifest *proto.Manifest) {
	protos := filter(manifest.BuildDependencies.Items, func(x string) bool {
		return strings.HasSuffix(x, ".proto")
	})

	paths := infra_shared.MakePaths(componentDir)
	root := infra_shared.ReadRootAtPath(paths.RootDir + "/root.textproto")
	repo := root.Repo

	if len(protos) > 0 {
		if err := os.MkdirAll(paths.WorkspaceDir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(paths.WorkspaceDir+"/Makefile", []byte(makefile.Build(manifest.Component, paths, paths.PrefixFromGenToRoot, protos)), 0644); err != nil {
			log.Fatal(err)
		}

		infra_shared.Run("cd " + paths.WorkspaceDir + " && make")
	}

	grpcServerManifest := manifest.Component.GetGrpcServer()

	rpcs := shared.FindServiceInfo(paths.RootDir, grpcServerManifest.Definition)

	if err := os.MkdirAll(paths.GenDir+"/cmd", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/manifest", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/mockmanifest", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/infra", os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/infra/Dockerfile", []byte(dockerfile.Build(paths, manifest)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/cmd/main.go", []byte(mainfile.Build(paths, manifest, rpcs, repo)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/manifest/manifest.go", []byte(manifestfile.Build(paths, manifest, repo)), 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(paths.GenDir+"/mockmanifest/manifest.go", []byte(mockmanifestfile.Build(paths, manifest, rpcs, repo)), 0644); err != nil {
		log.Fatal(err)
	}

	if manifest.Component.GetGrpcServer() != nil && manifest.Component.GetGrpcServer().Target.GetCluster() != nil {
		cluster := manifest.Component.GetGrpcServer().Target.GetCluster()

		deployment := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      serviceAccountName: %s
      nodeSelector:
        iam.gke.io/gke-metadata-server-enabled: "true"
      containers:
      - name: %s
        image: gcr.io/magicpantryio/%s-%s:latest
        ports:
        - containerPort: %d
        imagePullPolicy: Always
        env:
          - name: PORT
            value: "%d"`, cluster.Deployment, cluster.Replicas, cluster.Label, cluster.Label, cluster.ServiceAccount, cluster.Container, manifest.Component.Namespace, manifest.Component.Name, cluster.Port, cluster.Port)

		service := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  annotations:
    networking.gke.io/load-balancer-type: "%s"
spec:
  type: LoadBalancer
  externalTrafficPolicy: Cluster
  selector:
    app: %s
  ports:
  - port: %d
    targetPort: %d`, cluster.ServiceName, "Internal", cluster.Label, cluster.Port, cluster.Port)

		if err := os.WriteFile(paths.GenDir+"/infra/deployment.yaml", []byte(deployment), 0644); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(paths.GenDir+"/infra/service.yaml", []byte(service), 0644); err != nil {
			log.Fatal(err)
		}
	}

	for _, rpc := range rpcs {
		path := paths.ComponentDir + "/handlers/" + ToSnakeCase(rpc.Name) + ".go"
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(paths.ComponentDir+"/handlers", os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(grpcstub.Build(paths, grpcServerManifest, rpc, repo)), 0644); err != nil {
			log.Fatal(err)
		}
	}
	for _, rpc := range rpcs {
		path := paths.ComponentDir + "/handlers/" + ToSnakeCase(rpc.Name) + "_test.go"
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(paths.ComponentDir+"/handlers", os.ModePerm); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(grpcteststub.Build(paths, manifest, rpc, repo)), 0644); err != nil {
			log.Fatal(err)
		}
	}

	infra_shared.Run("cd " + paths.GenDir + "/manifest" + " && go fmt")
	infra_shared.Run("cd " + paths.GenDir + "/mockmanifest" + " && go fmt")
	infra_shared.Run("cd " + paths.GenDir + "/cmd" + " && go fmt")
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
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
