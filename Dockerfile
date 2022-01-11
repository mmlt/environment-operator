# Build the controller binary
FROM golang:1.16 as builder

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
COPY cmd/ cmd/
COPY controllers/ controllers/
COPY pkg/ pkg/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -ldflags "-X main.Version=$VERSION" -a -o manager main.go


# Bake image
FROM docker.io/ubuntu:20.04

ARG VERSION_TERRAFORM=1.1.3
ARG VERSION_KUBECTL_TMPLT=0.4.4
ARG VERSION_KUBECTL=v1.21.6
ARG VERSION_CUE=v0.4.0

RUN apt update \
 && apt install -y curl git jq unzip vim-tiny \
 && rm -rf /var/lib/apt/lists/*

# Install environment-operator
COPY --from=builder /workspace/manager /usr/local/bin/envop
COPY envopwrap /usr/local/bin/envopwrap

# Install Azure CLI
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash

# Install Terraform
RUN curl -Lo terraform.zip https://releases.hashicorp.com/terraform/${VERSION_TERRAFORM}/terraform_${VERSION_TERRAFORM}_linux_amd64.zip \
 && unzip terraform.zip \
 && rm terraform.zip \
 && mv terraform /usr/local/bin/terraform

# Install kubectl
RUN curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/${VERSION_KUBECTL}/bin/linux/amd64/kubectl \
 && chmod +x kubectl \
 && mv kubectl /usr/local/bin/kubectl

# Install kubectl-tmplt
RUN curl -L https://github.com/mmlt/kubectl-tmplt/releases/download/v${VERSION_KUBECTL_TMPLT}/kubectl-tmplt_${VERSION_KUBECTL_TMPLT}_linux_amd64.tar.gz | tar xz \
 && mv kubectl-tmplt /usr/local/bin/kubectl-tmplt

# Install CUE
RUN curl -L https://github.com/cuelang/cue/releases/download/${VERSION_CUE}/cue_${VERSION_CUE}_linux_amd64.tar.gz | tar xz \
  && mv cue /usr/local/bin/cue

# Create user
RUN groupadd envop -g 1000 \
 && useradd --gid 1000 -u 1000 -s /bin/bash -m envop #--groups sudo
USER 1000:1000
WORKDIR /home/envop



