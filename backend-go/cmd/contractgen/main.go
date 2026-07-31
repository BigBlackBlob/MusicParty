package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/contractspec"
)

func main() {
	check := flag.Bool("check", false, "fail when generated contract assets differ from the working tree")
	repository := flag.String("repo", "..", "MusicParty repository root")
	flag.Parse()
	root, err := filepath.Abs(*repository)
	if err != nil {
		fatal(err)
	}
	files, err := contractspec.Generate(root)
	if err != nil {
		fatal(err)
	}
	if *check {
		err = contractspec.Check(root, files)
	} else {
		err = contractspec.Write(root, files)
	}
	if err != nil {
		fatal(err)
	}
	fmt.Printf("contract assets verified: %d files\n", len(files))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
