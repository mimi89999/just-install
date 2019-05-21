package main

import (
	"log"

	"github.com/just-install/just-install/pkg/fetch"
	"github.com/just-install/just-install/pkg/justinstall"
	"github.com/just-install/just-install/pkg/paths"
	"github.com/urfave/cli"
)

func loadRegistry(c *cli.Context) justinstall.Registry {
	if !c.GlobalIsSet("registry") {
		return justinstall.SmartLoadRegistry(false)
	}

	// Load custom registry
	src := c.GlobalString("registry")

	log.Println("Loading custom registry at", src)

	dst, err := paths.TempFileCreate("custom-registry.json")
	if err != nil {
		log.Fatalln("Could not create temporary directory to download the custom registry:", err)
	}

	dst, err = fetch.Fetch(src, &fetch.Options{Destination: dst, Progress: true})
	if err != nil {
		log.Fatalln("Cannot obtain custom registry:", err)
	}

	return justinstall.LoadRegistry(dst)
}
