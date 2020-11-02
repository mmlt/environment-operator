# Build the controller binary
FROM golang:1.13 as builder

ARG VERSION

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy and build go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -ldflags "-X main.Version=$VERSION" -a -o manager main.go


# Bake image
FROM docker.io/ubuntu:20.04

ARG VERSION_TERRAFORM=0.12.24
ARG VERSION_KUBECTL_TMPLT=v0.4.0
ARG VERSION_KUBECTL=v1.18.1

RUN apt update \
 && apt install -y curl unzip \
 && rm -rf /var/lib/apt/lists/*

# Install environemnt-operator
COPY --from=builder /workspace/manager /usr/local/bin/envop

# Install Terraform
RUN curl -Lo terraform.zip https://releases.hashicorp.com/terraform/${VERSION_TERRAFORM}/terraform_${VERSION_TERRAFORM}_linux_amd64.zip \
 && unzip terraform.zip \
 && rm terraform.zip \
 && mv terraform /usr/local/bin/terraform

# Install kubectl-tmplt
RUN curl -Lo kubectl-tmplt https://github.com/mmlt/kubectl-tmplt/releases/download/${VERSION_KUBECTL_TMPLT}/kubectl-tmplt-${VERSION_KUBECTL_TMPLT}-linux-amd64 \
 && chmod +x kubectl-tmplt \
 && mv kubectl-tmplt /usr/local/bin/kubectl-tmplt

# Install kubectl
RUN curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/${VERSION_KUBECTL}/bin/linux/amd64/kubectl \
 && chmod +x kubectl \
 && mv kubectl /usr/local/bin/kubectl

WORKDIR /
USER 1000:1000


