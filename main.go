package main

import (
	"fmt"
	"github.com/mmlt/environment-operator/cmd"
	"os"
)

func main() {
	err := cmd.NewRootCommand().Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
