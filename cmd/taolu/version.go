package main

import (
	"fmt"

	"github.com/yli/taolu/pkg/version"
)

func runVersion() {
	fmt.Printf("taolu %s\n", version.Version)
}
