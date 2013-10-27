// Copyright 2013 Julian Phillips.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
)

const (
	maxNum   = 3656158440062976
	alphabet = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type HashWriter struct {
	h1    hash.Hash
	h256  hash.Hash
	h5    hash.Hash
	bytes int
	multi io.Writer
}

func NewHashWriter(w io.Writer) *HashWriter {
	hw := &HashWriter{
		h1:   sha1.New(),
		h256: sha256.New(),
		h5:   md5.New(),
	}
	hw.multi = io.MultiWriter(w, hw.h1, hw.h5, hw.h256)
	return hw
}

func (hw *HashWriter) Write(p []byte) (n int, err error) {
	n, err = hw.multi.Write(p)
	hw.bytes += n
	return
}

func (hw *HashWriter) Written() int {
	return hw.bytes
}

func (hw *HashWriter) Sha1() string {
	return hex.EncodeToString(hw.h1.Sum(nil))
}

func (hw *HashWriter) Sha256() string {
	return hex.EncodeToString(hw.h256.Sum(nil))
}

func (hw *HashWriter) Md5() string {
	return hex.EncodeToString(hw.h5.Sum(nil))
}

func base36(n int64) string {
	chars := []byte(alphabet)
	res := []byte("==========")
	for i := 9; i >= 0 && n > 0; i-- {
		res[i] = chars[n%36]
		n /= 36
	}
	return string(res)
}

func randNameGen(nameCh chan string) {
	used := make(map[string]bool, 10)
	path := filepath.Join(repoPath, "names.dat")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open '%s': %s\n", path, err)
		os.Exit(1)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		used[scanner.Text()] = true
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Failed to read '%s': %s\n", path, err)
		os.Exit(1)
	}
	for {
		num := rand.Int63n(maxNum)
		name := base36(num)
		_, found := used[name]
		if found {
			continue
		}
		nameCh <- name
		_, err := f.WriteString(fmt.Sprintf("%s\n", name))
		if err != nil {
			log.Printf("Failed to write '%s': %s\n", path, err)
			os.Exit(1)
		}
		err = f.Sync()
		if err != nil {
			log.Printf("Failed to sync '%s': %s\n", path, err)
			os.Exit(1)
		}
		log.Printf("GEN: %d -> %s\n", num, name)
	}
}

func saveUpload(prefix, name string, r io.Reader) (string, string, error) {
	dir, err := ioutil.TempDir(tmpPath, prefix+"-")
	if err != nil {
		log.Printf("Failed to create tmp directory: %s\n", err)
		return "", "", err
	}
	debPath := filepath.Join(dir, name)
	f, err := os.Create(debPath)
	if err != nil {
		log.Printf("Failed to create '%s': %s\n", debPath, err)
		return dir, "", err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	if err != nil {
		log.Printf("Failed to write data to '%s': %s\n", debPath, err)
		return dir, "", err
	}
	return dir, debPath, nil
}

func getDefaultKey() (string, error) {
	key, err := cfg.Get("default-key", "")
	if err != nil {
		log.Printf("Failed to load config 'default-key': %s\n", err)
		return "", err
	}
	if key == "" {
		log.Printf("Signing requested, but no key configured.\n")
		log.Printf("Please set 'default-key' in config.yml\n")
		return "", fmt.Errorf("default-key not set")
	}
	return key, nil
}
