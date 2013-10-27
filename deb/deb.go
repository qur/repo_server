// Copyright 2013 Julian Phillips.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"repo_server/opgp"

	"github.com/qur/ar"
	"github.com/qur/godebiancontrol"
)

type Deb struct {
	name string
	f    *os.File
}

type NotFound struct {
	d    *Deb
	name string
}

func (nf *NotFound) Error() string {
	return fmt.Sprintf("Section '%s' was not found in deb '%s'", nf.name, nf.d)
}

type InvalidDeb struct {
	d   *Deb
	err error
}

func (id *InvalidDeb) Error() string {
	return fmt.Sprintf("Deb file '%s' was not valid: %s", id.d.name, id.err)
}

func Open(filename string) (*Deb, error) {
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	d := &Deb{
		name: filename,
		f:    f,
	}
	err = d.validate()
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Deb) validate() error {
	_, err := d.f.Seek(0, 0)
	if err != nil {
		return &InvalidDeb{d, err}
	}
	rd := ar.NewReader(d.f)
	hdr, err := rd.Next()
	if err != nil {
		return &InvalidDeb{d, err}
	}
	hdr.Name = strings.Trim(hdr.Name, "/")
	if hdr.Name != "debian-binary" {
		err := fmt.Errorf("First file in .deb must be 'debian-binary', not '%s'", hdr.Name)
		return &InvalidDeb{d, err}
	}
	buf := make([]byte, 10)
	n, err := rd.Read(buf)
	if err != nil {
		return &InvalidDeb{d, err}
	}
	version := strings.TrimSpace(string(buf[:n]))
	if version != "2.0" {
		err := fmt.Errorf("Only v2.0 .deb files supported, not v%s", version)
		return &InvalidDeb{d, err}
	}
	_, err = d.f.Seek(0, 0)
	if err != nil {
		return &InvalidDeb{d, err}
	}
	return nil
}

func (d *Deb) findSection(name string) (io.Reader, error) {
	_, err := d.f.Seek(0, 0)
	if err != nil {
		return nil, &InvalidDeb{d, err}
	}
	rd := ar.NewReader(d.f)
	section := ""
	for section != name {
		hdr, err := rd.Next()
		if err == io.EOF {
			return nil, &NotFound{d, name}
		} else if err != nil {
			return nil, &InvalidDeb{d, err}
		}
		section = strings.Trim(hdr.Name, "/")
	}
	return rd, nil
}

func (d *Deb) Close() error {
	return d.f.Close()
}

func (d *Deb) Control(name string) ([]map[string]string, error) {
	f, err := d.findSection("control.tar.gz")
	if err != nil {
		return nil, err
	}
	g, err := gzip.NewReader(f)
	if err != nil {
		return nil, &InvalidDeb{d, err}
	}
	defer g.Close()
	t := tar.NewReader(g)
	filename := ""
	for filename != name {
		hdr, err := t.Next()
		if err == io.EOF {
			return nil, err
		} else if err != nil {
			return nil, &InvalidDeb{d, err}
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		filename = strings.TrimPrefix(hdr.Name, "./")
	}
	// t should now point at the appropriate control file ...
	paras, err := godebiancontrol.Parse(t)
	if err != nil {
		return nil, &InvalidDeb{d, err}
	}
	info := make([]map[string]string, len(paras))
	for i, para := range paras {
		info[i] = para
	}
	return info, nil
}

func (d *Deb) hashSections() (string, error) {
	h1 := sha1.New()
	h5 := md5.New()
	w := io.MultiWriter(h1, h5)

	_, err := d.f.Seek(0, 0)
	if err != nil {
		return "", &InvalidDeb{d, err}
	}

	s := ""

	rd := ar.NewReader(d.f)
	for {
		h1.Reset()
		h5.Reset()

		hdr, err := rd.Next()
		if err == io.EOF {
			return s, nil
		} else if err != nil {
			return "", &InvalidDeb{d, err}
		}

		_, err = io.Copy(w, rd)
		if err != nil {
			return "", &InvalidDeb{d, err}
		}

		h1x := hex.EncodeToString(h1.Sum(nil))
		h5x := hex.EncodeToString(h5.Sum(nil))
		s += fmt.Sprintf("\t%s %s %d %s\n", h5x, h1x, hdr.Size, hdr.Name)
	}
}

func (d *Deb) Sign(key string) error {
	_, err := d.findSection("_gpgbuilder")
	if _, ok := err.(*NotFound); !ok && err != nil {
		return err
	} else if err == nil {
		return fmt.Errorf("%s already signed.", d.name)
	}
	signer, err := opgp.GetSignerName(key)
	if err != nil {
		return err
	}
	s := "Version: 4\n"
	s += fmt.Sprintf("Signer: %s\n", signer)
	s += time.Now().Format("Date: Mon Jan 02 15:04:05 2006\n")
	s += "Role: builder\n"
	s += "Files:\n"
	s2, err := d.hashSections()
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	err = opgp.Clearsign(strings.NewReader(s + s2), buf, key)
	if err != nil {
		return err
	}
	_, err = d.f.Seek(0, 2)
	if err != nil {
		return err
	}
	wr := ar.NewWriter(d.f)
	hdr := &ar.Header{
		Name:    "_gpgbuilder",
		ModTime: time.Now(),
		Uid:     0,
		Gid:     0,
		Mode:    0644,
		Size:    int64(buf.Len()),
	}
	err = wr.WriteHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(wr, buf)
	if err != nil {
		return err
	}
	return nil
}
