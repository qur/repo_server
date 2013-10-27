// Copyright 2013 Julian Phillips.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net/http"
	"os"
	"flag"

	"repo_server/opgp"
)

var (
	repoPath  = "repos"
	filesPath = "files"
	tmpPath   = "tmp"
)

var cwd = flag.String("dir", ".", "Change to this directory before doing anything.")

var cfg *Config
var names = make(chan string)
var manageOnly = false

func loadConfig() string {
	cfgPath := "config.yml"
	if flag.NArg() > 0 {
		cfgPath = flag.Arg(0)
	}
	cfg = LoadConfig(cfgPath)
	listen, err := cfg.Get("listen", ":8080")
	if err != nil {
		log.Printf("Error loading config 'listen': %s\n", err)
		os.Exit(1)
	}
	manageOnly, err = cfg.GetBool("manage-only", manageOnly)
	if err != nil {
		log.Printf("Error loading config 'listen': %s\n", err)
		os.Exit(1)
	}
	opgp.KeyringFile, err = cfg.Get("keyring", opgp.KeyringFile)
	if err != nil {
		log.Printf("Error loading config 'keyring': %s\n", err)
		os.Exit(1)
	}
	cwd, err := cfg.Get("path.cwd", ".")
	if err != nil {
		log.Printf("Error loading config 'path.cwd': %s\n", err)
		os.Exit(1)
	}
	err = os.Chdir(cwd)
	if err != nil {
		log.Printf("Failed to set cwd to '%s': %s\n", cwd, err)
		os.Exit(1)
	}
	repoPath, err = cfg.Get("path.repos", repoPath)
	if err != nil {
		log.Printf("Error loading config 'path.repos': %s\n", err)
		os.Exit(1)
	}
	filesPath, err = cfg.Get("path.files", filesPath)
	if err != nil {
		log.Printf("Error loading config 'path.files': %s\n", err)
		os.Exit(1)
	}
	tmpPath, err = cfg.Get("path.tmp", tmpPath)
	if err != nil {
		log.Printf("Error loading config 'path.tmp': %s\n", err)
		os.Exit(1)
	}
	return listen
}

func prepPaths() {
	for _, dir := range []string{repoPath, filesPath, tmpPath} {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.Printf("Failed to create '%s': %s\n", dir, err)
			os.Exit(1)
		}
	}
}

func prepRepos() {
	repos, err := cfg.GetMapList("repos")
	if err != nil {
		log.Printf("Failed to read shared repo config: %s\n", err)
		os.Exit(1)
	}
	for i, entry := range repos {
		name, ok := entry["name"]
		if !ok {
			log.Printf("Repo entry %d, missing name!\n", i+1)
			os.Exit(1)
		}
		_, ok = entry["codename"]
		if !ok {
			log.Printf("Repo entry %d, missing codename!\n", i+1)
			os.Exit(1)
		}
		err = UpdateSharedRepo(name, entry)
		if err != nil {
			log.Printf("Failed to update Repo %s: %s\n", name, err)
			os.Exit(1)
		}
	}
}

func main() {
	flag.Parse()
	err := os.Chdir(*cwd)
	if err != nil {
		log.Printf("Failed to set cwd to '%s': %s\n", *cwd, err)
		os.Exit(1)
	}
	listen := loadConfig()
	prepPaths()
	prepRepos()
	go randNameGen(names)
	if !manageOnly {
		http.Handle("/", http.FileServer(http.Dir(filesPath)))
		http.Handle("/r/", http.StripPrefix("/r/", http.FileServer(http.Dir(repoPath))))
	}
	http.HandleFunc("/c/", handleControlRequest)
	log.Printf("-- start web server --\n")
	log.Fatal(http.ListenAndServe(listen, nil))
}
