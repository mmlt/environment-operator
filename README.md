# Environment operator

## Introduction

The environment-operator (envop) is a Kubernetes operator that:
- provides a declarative way to specify environments (with OpenAPI schema)
- adds the concept of "budgets", "schedules", "allowed steps" to constrain change
- can be build on top of by other tools, for example to do canary updates, recreate a cluster from back-up etc.


Multiple instances of the envop can be used, typically one per environment type (sandbox, dev, test, prod)
This allows for running different versions of envop for test and prod.


From a birds-eye view an envop instance gets a desired state specification (Environment Custom Resource), reads various (git) sources, looks-up secrets
and decides what steps it needs to take to bring the target environment in the desired state using tools like Terraform, az, kubectl and kubectl-tmplt.



				  +-----------------+
	    Environment CR +----->|                 |           terraform
				  |   environment   |-------->              +----> AzureRM
	    terraform code +----->|                 |               az
				  |    operator     |
	kubectl-tmplt code +----->|                 |             kubectl
				  |    (envop)      |-------->              +----> K8S API
	          KeyVault +----->|                 |          kubectl-tmplt
				  +-----------------+
				       ^   ^
		ServicePrincipal +-----+   |
					   |
		  GIT access key +---------+


The envop uses the following steps to work towards desired state:
- `Infra` create/updates/deletes infra resources using terraform.
- `AKSPool` updates the version of AKS pools
- `Kubectl` creates kubeconfig for other steps to use
- `AKSAddonPreflight` wait for an AKS cluster to be up
  - API Server reachable (AZ FW might block traffic until configured)
  - default StorageClasses present.
- `Addons` deploy kubernetes applications

As a special case envop can destroy an environment:
- `Destroy` destroy an environment
(to destroy an environment the `destroy` field must be `true` and `budget.deleteLimit` must be `99`)

A step is run as soon as their dependencies change. 
Dependencies include repo contents, Environment values and vault values referenced from Environment fields. 

When a step fails the corresponding Environment `status.steps.state` becomes `Error` and ` status.step.message` is updated with an explanation.
To retry the step use the `reset-step` command.


Under the hood envop uses terraform, az, kubectl, kubectl-tmplt and git to do the work.
This has the benefit that humans can use the CLI's to perform repair actions that envop is not capable of.


## Constrain changes

The envop can be contrained in the changes it can make.

With `--allowed-steps` the list of steps is specified that can be run.

In the Environment `budget`s can be specified. These are limits on the maximum number of resources that can be added, changed or deleted by the Infra step.
Setting all to 0 has the same effect as not having `Infra` in the list of allowed-steps; no Infra changes can be made.

Finally the environment.yaml can specify a schedule. This is a time period in which steps are allowed to run.


## Environment Custom Resource

The environment is specified by a Kubernetes Custom Resource.

The CR `spec.infra` specifies infra that is used by all clusters.
The CR `spec.clusters` specifies cluster specific config and contains config for zero or more clusters.
To reduce repetition common cluster values can be set under `defaults`.

Besides literal values fields like `state` and `az.aad` can reference KeyVault values. 
To use a value from vault specify the value in `"vault secretname optional-field-name"` format.
If the optional-field-name is present the vault secret must be a JSON string with that particular field name. 

The `infra` and `clusters` blocks each specify a `source` that refers to the code to use.
For `infra` this is terraform code and for `clusters` this is kubectl-tmplt code.
The source can be of type `local` meaning `url` points to a directory containing the code or it can be of type `git` where `url` refers to a GIT repository.


## Secrets

Each envop instance is configured with a ServicePrincipal (SP) and optionally a GIT access key.


The SP
- is allowed to read KeyVault to resolve vault secret references in the Environment CR
- is allowed to contribute to the target ResourceGroup
- is specified by `--credentials-file`

The `--credentials-file` value is a file path, the file contains the SP in JSON: `{"client_id":"c..6", "client_secret":"V..O", "tenant":"4..9"}`


The (optional) GIT SSH key allows envop to read repositories, it is expected ~/.ssh to be used by git cli.


## Development

Prerequisites
- kubebuilder in path (`export PATH=/usr/local/kubebuilder/bin/:$PATH`)


### Run local

For development an `environment.yaml` is applied to a local k8s cluster, the envop run in your IDE.

Prerequisites
- minikube or another local k8s cluster
- az, git, terraform, kubectl, kubectl-tmplt CLI's in $PATH (see Dockerfile)
 
Make sure kubectl context refers to your local cluster

    kubectl config current-context

Install CRD in cluster

    make generate install

Apply environment.yaml

    kubectl delete -f environment.yaml; kubectl apply -f environment.yaml

Run envop with the following flags

    --credentials-file=~/envop/sp
    --vault=foobar-12345678
    --workdir=/var/tmp/envop
    --selector=sandboxenv
    --alsologtostderr=true
    --metrics-addr=:8081
    -v=5

