package dockerfile

import (
	"fmt"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	infra_shared "github.com/magicpantry/infra/shared"
)

func Build(paths infra_shared.Paths, mf *proto.Manifest) string {
	cloudbuild := strings.Split(paths.ComponentDir, paths.RootDir+"/")[1]

	if mf.Component.GetModelServer() == nil {
		var otherEnv, otherRun, otherInstall string
		env := "ENV PORT 50051"
		port := " --port $PORT"

		if mf.Component.GetJob() != nil {
			port = ""
			env = ""
		}
		for _, dep := range mf.RuntimeDependencies.Items {
			if dep.GetChrome() != nil {
				otherEnv = "ENV DEBIAN_FRONTEND noninteractive"
				otherRun = strings.Join([]string{
					"RUN wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb",
					"RUN apt-get install -y ./google-chrome-stable_current_amd64.deb",
				}, "\n")
				otherInstall = " wget"
			}
		}

		var preEnv string
		if mf.Component.GetGrpcServer() != nil && mf.Component.GetGrpcServer().Target.GetCluster() != nil {
			/*
				preEnv = "TEST=$(curl \"https://google.com\") IP=$(curl \"https://ipinfo.io/ip\") && "
				otherInstall = " curl"
			*/
		}

		return fmt.Sprintf(template, otherEnv, otherInstall, otherRun, cloudbuild, mf.Component.Name, mf.Component.Name,
			env, preEnv, mf.Component.Name, port)
	} else {
		return fmt.Sprintf(modelTemplate, cloudbuild, strings.ReplaceAll(cloudbuild, "/", "."))
	}
}

const modelTemplate = `FROM python:3.11.3-slim-buster

COPY ./__init__.py ./__init__.py
COPY ./gen ./gen

RUN apt install ca-certificates && update-ca-certificates
RUN pip3 install numpy pillow protobuf grpcio grpcio-reflection tensorflow==2.13.0

ENV CONFIG_PATH "./gen/%s/cmd/config.json"
ENV PORT 50051

CMD python3 -m gen.%s.cmd --config $CONFIG_PATH --port $PORT
`

const template = `FROM debian:stable

%s

RUN apt update -y && apt-get upgrade -y
RUN apt-get install -y ca-certificates%s

%s

COPY .workspace/%s/bin/%s ./%s

%s

CMD %s./%s%s
`
