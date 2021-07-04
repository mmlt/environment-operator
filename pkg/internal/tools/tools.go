// +build tools

// Package tools lists the tools used by this project.
// Use 'make install-tools' to install the tools.
package tools

import (
	_ "github.com/securego/gosec/v2/cmd/gosec"
	_ "golang.org/x/tools/cmd/stringer"
	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
