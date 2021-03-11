package main

import (
	"bufio"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

// Based on https://github.com/tendermint/tendermint/blob/master/scripts/linkify_changelog.py
// This script goes through the provided file, and replaces any " \#<number>",
// with the valid markdown formatted link to it. e.g.
// " [\#number](https://github.com/livepeer/go-livepeer/pull/<number>)"
// Note that if the number is for a an issue, github will auto-redirect you when you click the link.
// It is safe to run the script multiple times in succession.
//
// Example usage $ go run cmd/scripts/linkify_changelog.go CHANGELOG_PENDING.md
func main() {
	if len(os.Args) < 2 {
		log.Fatal("Expected filename")
	}

	fname := os.Args[1]

	f, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	re, err := regexp.Compile(`\\#([0-9]*)`)
	if err != nil {
		log.Fatal(err)
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := re.ReplaceAllString(scanner.Text(), `[#$1](https://github.com/livepeer/go-livepeer/pull/$1)`)
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	if err := ioutil.WriteFile(fname, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		log.Fatal(err)
	}
}
