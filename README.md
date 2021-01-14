
## Development
Prerequisites
- kubebuilder in path (`export PATH=/usr/local/kubebuilder/bin/:$PATH`)

### Run
Run a dev version of envop using minikube for environment resource and a sandbox Azure ResourceGroup.

Prerequisites
- minikube or another local k8s cluster
- az, git, terraform, kubectl, kubectl-tmplt CLI's (see Dockerfile)
 
Make sure kubectl context refers to minikube

    kubectl config current-context

Install CRD in cluster

    make generate install

Apply environment.yaml

    kubectl delete -f environment.yaml
    kubectl apply -f environment.yaml

Run envop with the following flags

    --credentials-file=/var/envop/login/sp
    --vault=foobar-12345678
    --workdir=/var/tmp/envop
    --selector=playgroundenvs
    --alsologtostderr=true
    --metrics-addr=:8081
    -v=5

The credentials-file contains the envop ServicePrincipal ID, secret and tenant: `{"client_id":"c..6", "client_secret":"V..O", "tenant":"4..9"}`
