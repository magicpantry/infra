package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	gen_shared "github.com/magicpantry/infra/generate/shared"
	"github.com/magicpantry/infra/shared"
)

func main() {
	rootDir := shared.RootDir()
	repo := filepath.Base(rootDir)
	for componentDir, manifest := range shared.ReadManifest() {
		log.Printf("build '%s/%s'\n", manifest.Component.Namespace, manifest.Component.Name)
		runMain(repo, componentDir, manifest)
	}
}

func runMain(repo, componentDir string, mf *proto.Manifest) {
	if mf.Component.GetEndpoints() == nil {
		return
	}

	paths := shared.MakePaths(componentDir)

	serviceURL, ok := checkIfServiceExists(mf)
	if !ok {
		log.Println("service doesn't exist yet, making")

		out := shared.Run(fmt.Sprintf(
			"gcloud run deploy %s-%s --image=\"%s\" --allow-unauthenticated --region us-east1 --ingress internal --format json",
			mf.Component.Namespace, mf.Component.Name, "gcr.io/cloudrun/hello"))
		out = out[strings.Index(out, "{"):]

		var deploy deployJSON
		if err := json.Unmarshal([]byte(out), &deploy); err != nil {
			log.Fatal(err)
		}

		serviceURL = deploy.Status.Address.URL
	}

	cleaned := strings.Split(serviceURL, "https://")[1]

	otherManifest := shared.ReadManifestAtPath(paths.RootDir + "/" + mf.Component.GetEndpoints().Path + "/manifest.textproto")
	parts := strings.Split(otherManifest.Component.GetGrpcServer().Definition, ":")

	servicePackageParts := strings.Split(parts[0], "/")
	servicePackageParts = servicePackageParts[:len(servicePackageParts)-1]
	servicePackage := strings.Join(servicePackageParts, ".")
	serviceName := parts[1]

	apiConfig := fmt.Sprintf(
		apiConfigTemplate,
		cleaned,
		mf.Component.Namespace, mf.Component.Name,
		repo,
		servicePackage, serviceName,
		gen_shared.KeyOrValueToString(mf.Component.GetEndpoints().Domain, mf))

	protoPath := parts[0]
	pbPath := strings.ReplaceAll(protoPath, ".proto", ".pb")
	descriptorPath := paths.RootDir + "/gen/" + pbPath
	configPath := paths.GenDir + "/config/api_config.yaml"

	if err := os.MkdirAll(filepath.Dir(descriptorPath), os.ModePerm); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(paths.GenDir+"/config", os.ModePerm); err != nil {
		log.Fatal(err)
	}

	shared.Run(fmt.Sprintf(
		"protoc -I=%s -I=%s --include_imports --include_source_info --descriptor_set_out=%s %s",
		paths.RootDir+"/external", paths.RootDir,
		descriptorPath, paths.RootDir+"/"+protoPath))
	if err := os.WriteFile(configPath, []byte(apiConfig), 0644); err != nil {
		log.Fatal(err)
	}

	out := shared.Run(fmt.Sprintf(
		"gcloud endpoints services deploy %s %s --format json",
		descriptorPath, configPath))
	out = out[strings.Index(out, "{"):]
	var deployConfig deployConfigJSON
	if err := json.Unmarshal([]byte(out), &deployConfig); err != nil {
		log.Fatal(err)
	}

	out = strings.TrimSpace(shared.Run(fmt.Sprintf(
		"cd %s/util && ./gcloud_build_image -s %s -c %s -p magicpantryio",
		paths.RootDir, cleaned, deployConfig.ServiceConfig.ID)))
	parts = strings.Split(out, "\n")
	lastLine := parts[len(parts)-1]
	parts = strings.Fields(lastLine)

	container := parts[len(parts)-2]

	shared.Run(fmt.Sprintf(
		"gcloud run deploy %s-%s --image=\"%s\" --allow-unauthenticated --region us-east1 --ingress internal-and-cloud-load-balancing --set-env-vars=ESPv2_ARGS=--cors_preset=basic",
		mf.Component.Namespace, mf.Component.Name, container))
}

func checkIfServiceExists(mf *proto.Manifest) (string, bool) {
	log.Println("checking if service exists")

	out := shared.Run("gcloud run services list --format json")

	var deploys []deployJSON
	if err := json.Unmarshal([]byte(out), &deploys); err != nil {
		log.Fatal(err)
	}

	for _, deploy := range deploys {
		if deploy.Metadata.Name == fmt.Sprintf("%s-%s", mf.Component.Namespace, mf.Component.Name) {
			return deploy.Status.Address.URL, true
		}
	}

	return "", false
}

type deployConfigJSON struct {
	ServiceConfig serviceConfigJSON `json:"serviceConfig"`
}

type serviceConfigJSON struct {
	ID string `json:"id"`
}

type deployJSON struct {
	Metadata metadataJSON `json:"metadata"`
	Status   statusJSON   `json:"status"`
}

type metadataJSON struct {
	Name string `json:"name"`
}

type statusJSON struct {
	Address addressJSON `json:"address"`
}

type addressJSON struct {
	URL string `json:"url"`
}

const apiConfigTemplate = `# The configuration schema is defined by the service.proto file.
# https://github.com/googleapis/googleapis/blob/master/google/api/service.proto

type: google.api.Service
config_version: 3
name: %s
title: %s-%s
apis:
  - name: %s.%s.%s
usage:
  rules:
  - selector: "*"
    allow_unregistered_calls: true
backend:
  rules:
    - selector: "*"
      address: grpcs://%s
authentication:
  providers:
  - id: firebase
    jwks_uri: https://www.googleapis.com/service_accounts/v1/metadata/x509/securetoken@system.gserviceaccount.com
    issuer: https://securetoken.google.com/magicpantryio
    audiences: "magicpantryio"
  rules:
  - selector: "*"
    requirements:
      - provider_id: firebase`
