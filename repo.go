// Copyright 2013 Julian Phillips.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"repo_server/deb"
	"repo_server/opgp"
)

type Repo struct {
	Name     string              `json:"-"`
	Config   RepoConfig          `json:"config"`
	Packages RepoPackages        `json:"packages"`
	Files    map[string]RepoFile `json:"files"`
}

type RepoConfig struct {
	Origin      string `json:"origin"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Codename    string `json:"codename"`
	Component   string `json:"component"`
	Sign        bool   `json:"sign"`
	GpgKey      string `json:"gpgkey"`
}

type RepoFile struct {
	Size   uint64 `json:"size"`
	Sha1   string `json:"sha1"`
	Sha256 string `json:"sha256"`
	Md5    string `json:"md5"`
}

type RepoPackages struct {
	I386   PackageGroup `json:"i386"`
	Amd64  PackageGroup `json:"amd64"`
	Source PackageGroup `json:"source"`
}

type PackageGroup map[string]PackageSet

type PackageSet map[string]Package

type Package struct {
	Control     map[string]string `json:"control"`
	Description string            `json:"description"`
	Filename    string            `json:"filename"`
	Size        uint64            `json:"size"`
	Sha1        string            `json:"sha1"`
	Sha256      string            `json:"sha256"`
	Md5         string            `json:"md5"`
}

type PackageDetails map[string]map[string][]string

func newRepo(name string) *Repo {
	return &Repo{
		Name: name,
		Config: RepoConfig{
			Origin:      "<origin>",
			Label:       "<label>",
			Description: "<description>",
			Codename:    "<codename>",
			Component:   "main",
			Sign:        false,
			GpgKey:      "",
		},
		Packages: RepoPackages{
			I386:   make(PackageGroup),
			Amd64:  make(PackageGroup),
			Source: make(PackageGroup),
		},
		Files: make(map[string]RepoFile),
	}
}

func NewRepo() *Repo {
	return newRepo("@" + <-names)
}

func LoadRepo(name string) (*Repo, error) {
	r := newRepo(name)
	err := r.Load()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func UpdateSharedRepo(name string, settings map[string]string) error {
	var repo *Repo
	path := filepath.Join(repoPath, name)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		repo = newRepo(name)
	} else if err != nil {
		log.Printf("Failed to stat '%s': %s\n", repoPath, err)
		return err
	} else {
		repo, err = LoadRepo(name)
		if err != nil {
			return err
		}
	}
	val, ok := settings["origin"]
	if ok {
		repo.Config.Origin = val
	}
	val, ok = settings["label"]
	if ok {
		repo.Config.Label = val
	}
	val, ok = settings["description"]
	if ok {
		repo.Config.Description = val
	}
	val, ok = settings["codename"]
	if ok {
		repo.Config.Codename = val
	}
	val, ok = settings["component"]
	if ok {
		repo.Config.Component = val
	}
	val, ok = settings["sign"]
	if ok {
		repo.Config.Sign, err = strconv.ParseBool(val)
		if err != nil {
			return err
		}
	}
	if repo.Config.Sign {
		val, ok = settings["signing-key"]
		if ok {
			repo.Config.GpgKey = val
		} else {
			repo.Config.GpgKey, err = getDefaultKey()
			if err != nil {
				return err
			}
		}
	}
	return repo.Save()
}

func (r *Repo) Load() error {
	path := filepath.Join(repoPath, r.Name)
	metaPath := filepath.Join(path, ".meta")
	f, err := os.Open(metaPath)
	if err != nil {
		log.Printf("Failed to open '%s' file: %s\n", metaPath, err)
		return err
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(r)
	if err != nil {
		log.Printf("Failed to read '%s' file: %s\n", metaPath, err)
		return err
	}
	return nil
}

func (r *Repo) Save() error {
	path := filepath.Join(repoPath, r.Name)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		log.Printf("Failed to create repo directory: %s\n", err)
		return err
	}
	metaPath := filepath.Join(path, ".meta")
	f, err := os.Create(metaPath)
	if err != nil {
		log.Printf("Failed to open '%s' file: %s\n", metaPath, err)
		return err
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(r)
	if err != nil {
		log.Printf("Failed to read '%s' file: %s\n", metaPath, err)
		return err
	}
	err = r.writePackages()
	if err != nil {
		return err
	}
	return r.writeRelease()
}

func (r *Repo) storeHashes(path string, hw *HashWriter) {
	base := filepath.Join(repoPath, r.Name, "dists", r.Config.Codename)
	rel, err := filepath.Rel(base, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		log.Printf("Path '%s' wasn't under '%s'", path, base)
		return
	}
	r.Files[rel] = RepoFile{
		Size:   uint64(hw.Written()),
		Sha1:   hw.Sha1(),
		Sha256: hw.Sha256(),
		Md5:    hw.Md5(),
	}
}

func (r *Repo) signDeb(debPath string) error {
	if !r.Config.Sign {
		return nil
	}
	d, err := deb.Open(debPath)
	if err != nil {
		log.Printf("Failed to open deb '%s': %s\n", debPath, err)
		return err
	}
	defer d.Close()
	err = d.Sign(r.Config.GpgKey)
	if err != nil {
		log.Printf("Failed to sign deb '%s': %s\n", debPath, err)
		return err
	}
	return nil
}

func (r *Repo) parseDeb(debPath string) error {
	pkg := Package{}

	d, err := deb.Open(debPath)
	if err != nil {
		log.Printf("Failed to open deb '%s': %s\n", debPath, err)
		return err
	}
	defer d.Close()

	info, err := d.Control("control")
	if err != nil {
		log.Printf("Failed to parse deb '%s': %s\n", debPath, err)
		return err
	}

	if len(info) != 1 {
		log.Printf("%s: Expected 1 paragraph in .deb control file, not %d\n", debPath, len(info))
		return fmt.Errorf("%s/1 paragraphs in control: %s", len(info), debPath)
	}

	version := info[0]["Version"]
	pkgName := info[0]["Package"]
	arch := info[0]["Architecture"]

	pkg.Description = info[0]["Description"]
	pkg.Control = info[0]
	delete(pkg.Control, "Description")

	if version == "" {
		log.Printf("deb did not include version info: %s\n", debPath)
		return fmt.Errorf("no version in %s", debPath)
	}
	if pkgName == "" {
		log.Printf("deb did not include package name: %s\n", debPath)
		return fmt.Errorf("no package name in %s", debPath)
	}
	if arch == "" {
		log.Printf("deb did not include architecture: %s\n", debPath)
		return fmt.Errorf("no architecture in %s", debPath)
	}
	base := fmt.Sprintf("pool/%s/%s/%s/", r.Config.Component, pkgName[0:1], pkgName)
	debName := fmt.Sprintf("%s_%s_%s.deb", pkgName, version, arch)
	pkg.Filename = filepath.Join(base, debName)
	filename := filepath.Join(repoPath, r.Name, pkg.Filename)
	f, err := os.Open(debPath)
	if err != nil {
		log.Printf("Failed to open '%s': %s\n", debPath, err)
		return err
	}
	defer f.Close()
	destDir := filepath.Dir(filename)
	if destDir != "." {
		err = os.MkdirAll(destDir, 0755)
		if err != nil {
			log.Printf("Failed to create '%s': %s\n", destDir, err)
			return err
		}
	}
	w, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to open '%s': %s\n", filename, err)
		return err
	}
	hw := NewHashWriter(w)
	defer w.Close()
	size, err := io.Copy(hw, f)
	if err != nil {
		log.Printf("Failed to copy '%s' -> '%s': %s\n", debPath, filename, err)
		return err
	}
	pkg.Size = uint64(size)
	pkg.Sha1 = hw.Sha1()
	pkg.Sha256 = hw.Sha256()
	pkg.Md5 = hw.Md5()

	var pkgs map[string]PackageSet
	switch strings.ToLower(arch) {
	case "i386":
		pkgs = r.Packages.I386
	case "amd64":
		pkgs = r.Packages.Amd64
	case "source":
		pkgs = r.Packages.Source
	default:
		log.Printf("Unsupported architecture: %s\n", arch)
		return fmt.Errorf("Unsupported arch: %s", arch)
	}

	set, found := pkgs[pkgName]
	if !found {
		set = make(PackageSet)
	}
	set[version] = pkg
	pkgs[pkgName] = set

	return r.Save()
}

func (r *Repo) Add(debPath string) error {
	err := r.signDeb(debPath)
	if err != nil {
		return err
	}
	err = r.parseDeb(debPath)
	if err != nil {
		return err
	}
	return nil
}

func (pg PackageGroup) remove(name, version string) {
	set, found := pg[name]
	if !found {
		return
	}
	delete(set, version)
	if len(set) == 0 {
		delete(pg, name)
	}
}

func (r *Repo) Remove(name, version, arch string) error {
	switch arch {
	case "i386":
		r.Packages.I386.remove(name, version)
	case "amd64":
		r.Packages.Amd64.remove(name, version)
	case "source":
		r.Packages.Source.remove(name, version)
	default:
		log.Printf("Attempt to remove %s:%s from unknown arch: %s\n", name, version, arch)
		return nil
	}
	// remove file from pool
	debName := fmt.Sprintf("%s_%s_%s.deb", name, version, arch)
	path := filepath.Join(repoPath, r.Name, "pool", r.Config.Component, name[0:1], name, debName)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		log.Printf("Failed to delete %s from pool: %s\n", debName, err)
		return err
	}
	return nil
}

func (pg PackageGroup) packages(group string, packages PackageDetails) {
	for name := range pg {
		_, found := packages[name]
		if !found {
			packages[name] = make(map[string][]string)
		}
		for version := range pg[name] {
			packages[name][version] = append(packages[name][version], group)
		}
	}
}

func (r *Repo) ListPackages() PackageDetails {
	packages := make(PackageDetails)
	r.Packages.I386.packages("i386", packages)
	r.Packages.Amd64.packages("amd64", packages)
	r.Packages.Source.packages("source", packages)
	return packages
}

func (r *Repo) writePackages() error {
	err := r.Packages.I386.writePackages(r, "binary-i386")
	if err != nil {
		return err
	}
	err = r.writeDeepRelease("binary-i386", "i386")
	if err != nil {
		return err
	}
	err = r.Packages.Amd64.writePackages(r, "binary-amd64")
	if err != nil {
		return err
	}
	err = r.writeDeepRelease("binary-amd64", "amd64")
	if err != nil {
		return err
	}
	err = r.Packages.Source.writePackages(r, "source")
	if err != nil {
		return err
	}
	err = r.writeDeepRelease("source", "source")
	if err != nil {
		return err
	}
	return nil
}

func (r *Repo) writeRelease() error {
	path := filepath.Join(repoPath, r.Name, "dists", r.Config.Codename)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		log.Printf("Failed to create directory '%s': %s\n", path, err)
		return err
	}
	filename := filepath.Join(path, "Release")
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create Release file: %s\n", err)
		return err
	}
	defer f.Close()
	md5 := "MD5Sum:\n"
	sha1 := "SHA1:\n"
	sha256 := "SHA256:\n"
	for name := range r.Files {
		file := r.Files[name]
		md5 += fmt.Sprintf(" %s %d %s\n", file.Md5, file.Size, name)
		sha1 += fmt.Sprintf(" %s %d %s\n", file.Sha1, file.Size, name)
		sha256 += fmt.Sprintf(" %s %d %s\n", file.Sha256, file.Size, name)
	}
	s := fmt.Sprintf("Origin: %s\n", r.Config.Origin)
	s += fmt.Sprintf("Label: %s\n", r.Config.Label)
	s += fmt.Sprintf("Codename: %s\n", r.Config.Codename)
	s += fmt.Sprintf("Date: %s\n", time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 MST"))
	s += fmt.Sprintf("Architectures: i386 amd64\n")
	s += fmt.Sprintf("Components: %s\n", r.Config.Component)
	s += fmt.Sprintf("Description: %s\n", r.Config.Description)
	_, err = f.WriteString(s + md5 + sha1 + sha256)
	if err != nil {
		log.Printf("Failed to write %s: %s\n", path, err)
		return err
	}
	if !r.Config.Sign {
		return nil
	}
	gpgFilename := filepath.Join(path, "Release.gpg")
	err = opgp.SignFile(filename, gpgFilename, r.Config.GpgKey)
	if err != nil {
		return err
	}
	inFilename := filepath.Join(path, "InRelease")
	return opgp.ClearsignFile(filename, inFilename, r.Config.GpgKey)
}

func (r *Repo) writeDeepRelease(name, arch string) error {
	path := filepath.Join(repoPath, r.Name, "dists", r.Config.Codename, r.Config.Component, name)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		log.Printf("Failed to create directory '%s': %s\n", path, err)
		return err
	}
	filename := filepath.Join(path, "Release")
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create Release file: %s\n", err)
		return err
	}
	defer f.Close()
	hw := NewHashWriter(f)
	s := fmt.Sprintf("Component: %s\n", r.Config.Component)
	s += fmt.Sprintf("Origin: %s\n", r.Config.Origin)
	s += fmt.Sprintf("Label: %s\n", r.Config.Label)
	s += fmt.Sprintf("Architecture: %s\n", arch)
	s += fmt.Sprintf("Description: %s\n", r.Config.Description)
	_, err = hw.Write([]byte(s))
	if err != nil {
		log.Printf("Failed to write %s: %s\n", path, err)
		return err
	}
	r.storeHashes(filename, hw)
	return nil
}

func (pg PackageGroup) writePackages(r *Repo, name string) error {
	path := filepath.Join(repoPath, r.Name, "dists", r.Config.Codename, r.Config.Component, name)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		log.Printf("Failed to create directory '%s': %s\n", path, err)
		return err
	}
	file := "Packages"
	if name == "source" {
		file = "Sources"
	}
	filename := filepath.Join(path, file)
	gzFilename := filepath.Join(path, file) + ".gz"
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create Packages file: %s\n", err)
		return err
	}
	defer f.Close()
	hw := NewHashWriter(f)
	f2, err := os.Create(gzFilename)
	if err != nil {
		log.Printf("Failed to create Packages file: %s\n", err)
		return err
	}
	defer f2.Close()
	g := gzip.NewWriter(f2)
	defer g.Close()
	gzHw := NewHashWriter(g)
	w := io.MultiWriter(hw, gzHw)
	for name := range pg {
		for version := range pg[name] {
			pkg := pg[name][version]
			err := pkg.appendTo(w)
			if err != nil {
				return err
			}
		}
	}
	r.storeHashes(filename, hw)
	r.storeHashes(gzFilename, gzHw)
	return nil
}

func (p *Package) appendTo(w io.Writer) error {
	extra := ""
	for name, value := range p.Control {
		extra += fmt.Sprintf("%s: %s\n", name, value)
	}
	extra += fmt.Sprintf("Filename: %s\n", p.Filename)
	extra += fmt.Sprintf("Size: %d\n", p.Size)
	extra += fmt.Sprintf("SHA1: %s\n", p.Sha1)
	extra += fmt.Sprintf("SHA256: %s\n", p.Sha256)
	extra += fmt.Sprintf("MD5Sum: %s\n", p.Md5)
	extra += fmt.Sprintf("Description: %s\n", p.Description)
	extra += "\n"
	_, err := w.Write([]byte(extra))
	if err != nil {
		log.Printf("Failed to append blank line after %s: %s\n", p.Filename, err)
		return err
	}
	return nil
}
