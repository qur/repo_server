// Copyright 2013 Julian Phillips.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"repo_server/opgp"
)

func argCountOk(n int, args []string, w http.ResponseWriter, req *http.Request) bool {
	if len(args) == n+1 {
		return true
	}
	log.Printf("ERROR: got %d/%d args\n", len(args)-1, n)
	http.Error(w, "406: Not Acceptable", http.StatusNotAcceptable)
	return false
}

func handleControlRequest(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	log.Printf("Control request: %s\n", req.URL.Path)
	if len(req.URL.Path) < 4 {
		http.NotFound(w, req)
		return
	}
	bits := strings.Split(strings.Trim(req.URL.Path[3:], "/"), "/")
	command := strings.ToLower(bits[0])
	log.Printf("Command: %s\n", command)
	switch command {
	case "list":
		if argCountOk(0, bits, w, req) {
			listRepos(w, req)
		}
	case "create":
		if argCountOk(0, bits, w, req) {
			createRepo(w, req)
		}
	case "delete":
		if argCountOk(1, bits, w, req) {
			deleteRepo(bits[1], w, req)
		}
	case "include":
		if argCountOk(2, bits, w, req) {
			include(bits[1], bits[2], w, req)
		}
	case "remove":
		if argCountOk(1, bits, w, req) {
			remove(bits[1], w, req)
		}
	case "key":
		if argCountOk(1, bits, w, req) {
			genKey(bits[1], w, req)
		}
	case "packages":
		if argCountOk(1, bits, w, req) {
			listPackages(bits[1], w, req)
		}
	default:
		http.NotFound(w, req)
	}
}

type ListResp struct {
	Repos map[string]*RepoConfig `json:"repos"`
}

func listRepos(w http.ResponseWriter, req *http.Request) {
	resp := ListResp{
		Repos: make(map[string]*RepoConfig, 10),
	}
	files, err := ioutil.ReadDir(repoPath)
	if err != nil {
		log.Printf("Failed to ReadDir(%s): %s\n", repoPath, err)
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
	}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		repo, err := LoadRepo(file.Name())
		if err == nil {
			resp.Repos[file.Name()] = &repo.Config
		}
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Printf("Failed to encode JSON list response: %s\n", err)
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
	}
}

type CreateResp struct {
	Name string `json:"name"`
}

func createRepo(w http.ResponseWriter, req *http.Request) {
	repo := NewRepo()
	err := json.NewDecoder(req.Body).Decode(&repo.Config)
	if err != nil {
		log.Printf("Failed to decode JSON create request: %s\n", err)
		http.Error(w, "400: Create JSON Invalid", http.StatusBadRequest)
		return
	}
	if repo.Config.Sign {
		repo.Config.GpgKey, err = getDefaultKey()
		if err != nil {
			http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	err = repo.Save()
	if err != nil {
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(CreateResp{repo.Name})
	if err != nil {
		log.Printf("Failed to encode JSON create response: %s\n", err)
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
	}
}

func deleteRepo(name string, w http.ResponseWriter, req *http.Request) {
	if !strings.HasPrefix(name, "@") {
		log.Printf("Attempt to delete non temporary name: %s\n", name)
		http.Error(w, "403: Forbidden", http.StatusForbidden)
		return
	}
	path := filepath.Join(repoPath, name)
	err := os.RemoveAll(path)
	if err != nil {
		log.Printf("Failed to delete repo '%s': %s\n", name, err)
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func include(name, debName string, w http.ResponseWriter, req *http.Request) {
	repo, err := LoadRepo(name)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	dir, debPath, err := saveUpload(name, debName, req.Body)
	if dir != "" {
		defer os.RemoveAll(dir)
	}
	if err != nil {
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = repo.Add(debPath)
	if err != nil {
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type RemoveReq struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Arches  []string `json:"arches"`
}

func remove(name string, w http.ResponseWriter, req *http.Request) {
	repo, err := LoadRepo(name)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	rem := RemoveReq{}
	err = json.NewDecoder(req.Body).Decode(&rem)
	if err != nil {
		log.Printf("Failed to decode JSON remove request: %s\n", err)
		http.Error(w, "400: Create JSON Invalid", http.StatusBadRequest)
		return
	}
	if rem.Name == "" || rem.Version == "" {
		log.Printf("Invalid remove request: %+v", rem)
		http.Error(w, "400: Create JSON incomplete", http.StatusBadRequest)
		return
	}
	if len(rem.Arches) == 0 {
		rem.Arches = []string{"i386", "amd64", "source"}
	}
	for _, arch := range rem.Arches {
		err := repo.Remove(rem.Name, rem.Version, arch)
		if err != nil {
			http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	err = repo.Save()
	if err != nil {
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type KeyResp struct {
	Id       string `json:"id"`
	Filename string `json:"filename"`
}

func genKey(name string, w http.ResponseWriter, req *http.Request) {
	r, err := LoadRepo(name)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	if !r.Config.Sign {
		http.Error(w, "400: Bad Request", http.StatusBadRequest)
		return
	}
	keyName := fmt.Sprintf("%s.gpg.key", r.Config.GpgKey)
	keyPath := filepath.Join(filesPath, keyName)
	err = opgp.ExportKey(r.Config.GpgKey, keyPath)
	if err != nil {
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(KeyResp{r.Config.GpgKey, keyName})
	if err != nil {
		log.Printf("Failed to encode JSON key response: %s\n", err)
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
	}
}

type ListPkgsResp struct {
	Packages PackageDetails `json:"packages"`
}

func listPackages(name string, w http.ResponseWriter, req *http.Request) {
	r, err := LoadRepo(name)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	packages := r.ListPackages()
	err = json.NewEncoder(w).Encode(ListPkgsResp{packages})
	if err != nil {
		log.Printf("Failed to encode JSON key response: %s\n", err)
		http.Error(w, "500: Internal Server Error", http.StatusInternalServerError)
	}
}
