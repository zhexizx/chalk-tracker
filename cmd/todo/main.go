/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"chalk-tracker/internal/cli"
	"os"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
