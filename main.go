package main

import (
	"os"

	"github.com/kronostechnologies/richman/cmd"
)

var version = "latest"

const kubeFolder = "/.kube/config"

func main() {

	cmd.SetVersion(version)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
