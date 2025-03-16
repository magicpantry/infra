package terraform

import (
	"fmt"
	"strings"

	"github.com/magicpantry/infra/gen/proto"
	infra_shared "github.com/magicpantry/infra/shared"
)

func Build(rootDir string, mfMap map[string]*proto.Manifest, repo string) string {
	var resources []string
	for path, mf := range mfMap {
		cloudbuild := strings.Split(path, rootDir+"/")[1]
		var prefixArray []string
		for i := 0; i < len(strings.Split(cloudbuild, "/")); i++ {
			prefixArray = append(prefixArray, "..")
		}
		prefix := strings.Join(prefixArray, "/")
		var inner string

		var includes []string
		if mf.Component.GetEndpoints() != nil {
			otherManifest := infra_shared.ReadManifestAtPath(rootDir + "/" + mf.Component.GetEndpoints().Path + "/manifest.textproto")
			for _, include := range otherManifest.BuildDependencies.Items {
				if !strings.HasSuffix(include, ".proto") {
					continue
				}
				includes = append(includes, "\t\""+include+"\",")
			}
			inner = fmt.Sprintf(endpointsTemplate, mf.Component.Namespace, mf.Component.Name, prefix)
		}
		if mf.Component.GetJob() != nil {
			inner = fmt.Sprintf(
				jobTemplate,
				cloudbuild, prefix, cloudbuild, prefix,
				cloudbuild, mf.Component.Namespace, mf.Component.Name,
				cloudbuild, mf.Component.Namespace, mf.Component.Name,
				mf.Component.Namespace, mf.Component.Name,
				mf.Component.Namespace, mf.Component.Name)
		}
		if mf.Component.GetHttpServer() != nil {
			inner = fmt.Sprintf(
				httpServerTemplate,
				cloudbuild, prefix, cloudbuild, prefix,
				cloudbuild, mf.Component.Namespace, mf.Component.Name,
				cloudbuild, mf.Component.Namespace, mf.Component.Name,
				mf.Component.Namespace, mf.Component.Name,
				mf.Component.Namespace, mf.Component.Name)
		}
		if mf.Component.GetModelServer() != nil {
			auth := ""
			if mf.Component.GetModelServer().Target.GetRun() != nil && mf.Component.GetModelServer().Target.GetRun().Secure {
				auth = "--no-allow-unauthenticated"
			} else {
				auth = "--allow-unauthenticated"
			}

			if mf.Component.GetModelServer().Target.GetRun() != nil {
				inner = fmt.Sprintf(
					modelRunTemplate,
					cloudbuild, prefix,
					cloudbuild, mf.Component.Namespace, mf.Component.Name,
					cloudbuild, mf.Component.Namespace, mf.Component.Name,
					mf.Component.Namespace, mf.Component.Name,
					mf.Component.Namespace, mf.Component.Name,
					auth)
			} else {
				cluster := mf.Component.GetModelServer().Target.GetCluster()
				inner = fmt.Sprintf(
					modelClusterTemplate,
					cloudbuild, prefix,
					cloudbuild, mf.Component.Namespace, mf.Component.Name,
					cloudbuild, mf.Component.Namespace, mf.Component.Name,
					cloudbuild, mf.Component.Namespace, mf.Component.Name, cluster.Name)
			}
		}
		if mf.Component.GetGrpcServer() != nil {
			secrets := ""
			var secretArray []string
			for _, secret := range mf.Component.GetGrpcServer().Secrets {
				secretArray = append(secretArray, fmt.Sprintf("%s=%s:latest", secret.Value, secret.Value))
			}
			secrets = fmt.Sprintf("\n\t\"--update-secrets=%s\",", strings.Join(secretArray, ","))
			auth := ""
			if mf.Component.GetGrpcServer().Target.GetRun() != nil && mf.Component.GetGrpcServer().Target.GetRun().Secure {
				auth = "--no-allow-unauthenticated"
			} else {
				auth = "--allow-unauthenticated"
			}

			ingress := "all"
			if mf.Component.GetGrpcServer().Internal {
				ingress = "internal"
			}

			if mf.Component.GetGrpcServer().Target.GetRun() != nil {
				inner = fmt.Sprintf(
					grpcServerRunTemplate,
					cloudbuild, prefix, cloudbuild, prefix,
					cloudbuild, mf.Component.Namespace, mf.Component.Name,
					cloudbuild, mf.Component.Namespace, mf.Component.Name,
					mf.Component.Namespace, mf.Component.Name,
					secrets,
					mf.Component.Namespace, mf.Component.Name,
					auth, ingress)
			} else {
				cluster := mf.Component.GetGrpcServer().Target.GetCluster()
				inner = fmt.Sprintf(
					grpcServerClusterTemplate,
					cloudbuild, prefix, cloudbuild, prefix,
					cloudbuild, mf.Component.Namespace, mf.Component.Name,
					cloudbuild, mf.Component.Namespace, mf.Component.Name,
					cloudbuild, mf.Component.Namespace, mf.Component.Name, cluster.Name)
			}
		}
		if mf.Component.GetFunction() != nil {
			inner = fmt.Sprintf(
				functionTemplate,
				cloudbuild, prefix,
				mf.Component.Namespace, mf.Component.Name,
				mf.Component.Namespace, mf.Component.Name,
				"google.cloud.firestore.document.v1.deleted",
				mf.Component.GetFunction().Database,
				mf.Component.GetFunction().Collection,
			)
		}
		if mf.Component.GetWebapp() != nil {
			inner = fmt.Sprintf(
				webAppTemplate,
				cloudbuild, prefix, cloudbuild, repo, repo, repo, cloudbuild, repo)
		}

		if mf.RuntimeDependencies != nil {
			for _, item := range mf.RuntimeDependencies.Items {
				if item.GetHttpClient() != nil {
					otherManifest := infra_shared.ReadManifestAtPath(rootDir + "/" + item.GetHttpClient().Path + "/manifest.textproto")
					for _, include := range otherManifest.BuildDependencies.Items {
						if !strings.HasSuffix(include, ".proto") {
							continue
						}
						includes = append(includes, "\t\""+include+"\",")
					}
				}
			}
		}

		if mf.BuildDependencies != nil {
			for _, include := range mf.BuildDependencies.Items {
				if !strings.HasSuffix(include, ".proto") {
					include += "/**"
				}
				includes = append(includes, "\t\""+include+"\",")
			}
		}
		includes = append(includes, "\t\""+cloudbuild+"/**\",")

		resources = append(resources, fmt.Sprintf(
			template,
			mf.Component.Namespace, mf.Component.Name,
			mf.Component.Namespace, mf.Component.Name,
			strings.Join(includes, "\n"),
			repo, inner))
		if mf.Component.GetJob() != nil {
			resources = append(resources, fmt.Sprintf(
				jobTriggerTemplate,
				mf.Component.Namespace, mf.Component.Name,
				mf.Component.Namespace, mf.Component.Name,
				mf.Component.Namespace, mf.Component.Name,
				mf.Component.GetJob().Schedule,
				mf.Component.Namespace, mf.Component.Name))
		}
	}

	return providerTemplate + "\n" + strings.TrimSpace(strings.Join(resources, "\n"))
}

const providerTemplate = `
provider "google" {
	project = "magicpantryio"
	region = "us-east1"
	zone = "us-east1-b"
}

resource "random_id" "bucket_prefix" {
	byte_length = 8
}

resource "google_storage_bucket" "default" {
	name = "${random_id.bucket_prefix.hex}-bucket-tfstate"
	force_destroy = false
	location = "US"
	storage_class = "STANDARD"
	versioning {
		enabled = true
	}
}

resource "local_file" "default" {
  file_permission = "0644"
  filename        = "${path.module}/backend.tf"

  # You can store the template in a file and use the templatefile function for
  # more modularity, if you prefer, instead of storing the template inline as
  # we do here.
  content = <<-EOT
  terraform {
    backend "gcs" {
      bucket = "${google_storage_bucket.default.name}"
    }
  }
  EOT
}
`

const jobTriggerTemplate = `
resource "google_cloud_scheduler_job" "prod-%s-%s-job" {
    provider = google-beta
    name = "prod-%s-%s-job"
    description = "trigger %s-%s"
    schedule = "%s"
    attempt_deadline = "1800s"
    region = "us-east1"
    project = "magicpantryio"

    retry_config {
        retry_count = 3
    }

    http_target {
        http_method = "POST"
        uri = "https://us-east1-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/magicpantryio/jobs/%s-%s:run"

        oauth_token {
            service_account_email = "800468010335-compute@developer.gserviceaccount.com"
        }
    }
}
`

const template = `
resource "google_cloudbuild_trigger" "prod-%s-%s-trigger" {
  name = "prod-%s-%s-trigger"
  location = "global"

  included_files = [
%s
  ]

  github {
    owner = "magicpantry"
    name = "%s"

    push {
      branch = "^prod$"
    }
  }

  build {
    timeout = "1800s"

%s
  }
}`

const webAppTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/generate"
      id = "build-infra-gen"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "gen"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/generate/bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "build"
      script = "#!/usr/bin/env bash\nset -e\nnpm install --legacy-peer-deps && ng build --configuration production"
    }

    step {
      name = "gcr.io/cloud-builders/gsutil"
      id = "deploy"
      script = "#!/usr/bin/env bash\nset -e\ngsutil rm gs://magicpantry-app/main.*.js || true\ngsutil rm gs://%s-app/polyfills.*.js || true\ngsutil rm gs://%s-app/runtime.*.js || true\ngsutil rm gs://%s-app/styles.*.css || true\ngsutil -m cp -r /workspace/%s/dist/magicpantryio/* gs://%s-app/"
    }`

const jobTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/generate"
      id = "build-infra-gen"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/build"
      id = "build-infra-build"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "gen"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/generate/bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "build"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/build/bin/run"
    }

    step {
      name = "docker:stable"
      id = "build-image"
      args = ["build", ".", "-f", "gen/%s/infra/Dockerfile", "-t", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "docker:stable"
      id = "push-image"
      dir = "gen/%s/infra"
      args = ["push", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "gcr.io/cloud-builders/gcloud"
      id = "deploy"
      args = [
        "beta",
        "run",
        "jobs",
        "deploy",
        "%s-%s",
        "--image",
        "gcr.io/magicpantryio/%s-%s:latest",
        "--vpc-connector",
        "east-serverless",
        "--region",
        "us-east1",
        "--memory",
        "1Gi"
      ]
    }`

const httpServerTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/generate"
      id = "build-infra-gen"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/build"
      id = "build-infra-build"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "gen"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/generate/bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "build"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/build/bin/run"
    }

    step {
      name = "docker:stable"
      id = "build-image"
      args = ["build", ".", "-f", "gen/%s/infra/Dockerfile", "-t", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "docker:stable"
      id = "push-image"
      dir = "gen/%s/infra"
      args = ["push", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "gcr.io/cloud-builders/gcloud"
      id = "deploy"
      args = [
        "run",
        "deploy",
        "%s-%s",
        "--image",
        "gcr.io/magicpantryio/%s-%s:latest",
        "--region",
        "us-east1",
        "--allow-unauthenticated",
        "--ingress",
        "internal-and-cloud-load-balancing",
      ]
    }`

const modelRunTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/generate"
      id = "build-infra-gen"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "gen"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/generate/bin/run"
    }

    step {
      name = "docker:stable"
      id = "build-image"
      args = ["build", ".", "-f", "gen/%s/infra/Dockerfile", "-t", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "docker:stable"
      id = "push-image"
      dir = "gen/%s/infra"
      args = ["push", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "gcr.io/cloud-builders/gcloud"
      id = "deploy"
      args = [
        "run",
        "deploy",
        "%s-%s",
        "--image",
        "gcr.io/magicpantryio/%s-%s:latest",
        "--region",
        "us-east1",
        "--use-http2",
        "%s",
        "--ingress",
        "all",
        "--vpc-connector",
        "east-serverless",
        "--memory",
        "1Gi"
      ]
    }`

const modelClusterTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/generate"
      id = "build-infra-gen"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "gen"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/generate/bin/run"
    }

    step {
      name = "docker:stable"
      id = "build-image"
      args = ["build", ".", "-f", "gen/%s/infra/Dockerfile", "-t", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "docker:stable"
      id = "push-image"
      dir = "gen/%s/infra"
      args = ["push", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "gcr.io/cloud-builders/gke-deploy"
      id = "deploy"
      env = ["KUBECONFIG=/tmp/kubeconfig"]
      args = [
        "run",
        "--filename",
        "gen/%s/infra",
        "--image",
        "gcr.io/magicpantryio/%s-%s:latest",
        "--location",
        "us-east1-b",
        "--cluster",
        "%s",
      ]
    }`

const grpcServerClusterTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/generate"
      id = "build-infra-gen"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/build"
      id = "build-infra-build"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "gen"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/generate/bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "build"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/build/bin/run"
    }

    step {
      name = "docker:stable"
      id = "build-image"
      args = ["build", ".", "-f", "gen/%s/infra/Dockerfile", "-t", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "docker:stable"
      id = "push-image"
      dir = "gen/%s/infra"
      args = ["push", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "gcr.io/cloud-builders/gke-deploy"
      id = "deploy"
      env = ["KUBECONFIG=/tmp/kubeconfig"]
      args = [
        "run",
        "--filename",
        "gen/%s/infra",
        "--image",
        "gcr.io/magicpantryio/%s-%s:latest",
        "--location",
        "us-east1-b",
        "--cluster",
        "%s",
      ]
    }`

const grpcServerRunTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/generate"
      id = "build-infra-gen"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/build"
      id = "build-infra-build"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "gen"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/generate/bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "build"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/build/bin/run"
    }

    step {
      name = "docker:stable"
      id = "build-image"
      args = ["build", ".", "-f", "gen/%s/infra/Dockerfile", "-t", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "docker:stable"
      id = "push-image"
      dir = "gen/%s/infra"
      args = ["push", "gcr.io/$PROJECT_ID/%s-%s"]
    }

    step {
      name = "gcr.io/cloud-builders/gcloud"
      id = "deploy"
      args = [
        "run",
        "deploy",
        "%s-%s",%s
        "--image",
        "gcr.io/magicpantryio/%s-%s:latest",
        "--region",
        "us-east1",
        "--use-http2",
        "%s",
        "--ingress",
        "%s",
        "--vpc-connector",
        "east-serverless",
      ]
    }`

const functionTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/generate"
      id = "build-infra-gen"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      id = "gen-protos"
      script = "#!/usr/bin/env bash\nset -e\ninfra/generate/bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s"
      id = "gen"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/generate/bin/run"
    }

    step {
      name = "gcr.io/cloud-builders/gcloud"
      id = "deploy"
      args = [
        "functions",
        "deploy",
        "%s-%s",
        "--gen2",
        "--runtime",
        "go121",
        "--region",
        "us-east1",
        "--source=.",
        "--entry-point=%s-%s",
        "--trigger-event-filters=type=%s",
        "--trigger-event-filters=database=%s",
        "--trigger-event-filters-path-pattern=document=%s/{field}",
        "--no-allow-unauthenticated"
      ]
    }`

const endpointsTemplate = `    step {
        name = "gcr.io/cloud-builders/git"
        args = ["clone", "https://github.com/magicpantry/infra"]
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra"
      id = "make-infra"
      script = "#!/usr/bin/env bash\nset -e\nmake"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "infra/endpoints"
      id = "build-infra-endpoints"
      script = "#!/usr/bin/env bash\nset -e\nmkdir -p bin && go build -o bin/run"
    }

    step {
      name = "gcr.io/$PROJECT_ID/build:latest"
      dir = "%s/%s"
      id = "run-endpoints"
      script = "#!/usr/bin/env bash\nset -e\n%s/infra/endpoints/bin/run"
    }`
