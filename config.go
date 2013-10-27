// Copyright 2013 Julian Phillips.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strconv"

	"github.com/kylelemons/go-gypsy/yaml"
)

type Config struct {
	file *yaml.File
}

func LoadConfig(name string) *Config {
	return &Config{
		file: yaml.ConfigFile(name),
	}
}

func (c *Config) get(name, def string) (string, bool, error) {
	if c.file == nil {
		return def, false, nil
	}
	s, err := c.file.Get(name)
	if err == nil {
		return s, true, nil
	} else if _, ok := err.(*yaml.NodeNotFound); ok {
		return def, false, nil
	}
	return "", false, err
}

func (c *Config) Get(name, def string) (string, error) {
	s, _, err := c.get(name, def)
	if err != nil {
		return "", err
	}
	return s, nil
}

func (c *Config) GetBool(name string, def bool) (bool, error) {
	s, found, err := c.get(name, "")
	if err != nil {
		return false, err
	}
	v := def
	if found {
		v, err = strconv.ParseBool(s)
		if err != nil {
			return false, err
		}
	}
	return v, nil
}

func (c *Config) getUint(name string, def uint64, bits int) (uint64, error) {
	s, found, err := c.get(name, "")
	if err != nil {
		return 0, err
	}
	v := def
	if found {
		v, err = strconv.ParseUint(s, 10, bits)
		if err != nil {
			return 0, err
		}
	}
	return v, nil
}

func (c *Config) GetUint16(name string, def uint16) (uint16, error) {
	u, err := c.getUint(name, uint64(def), 16)
	return uint16(u), err
}

func (c *Config) GetUint64(name string, def uint64) (uint64, error) {
	return c.getUint(name, def, 64)
}

func (c *Config) GetMapList(name string) ([]map[string]string, error) {
	node, err := yaml.Child(c.file.Root, name)
	if _, ok := err.(*yaml.NodeNotFound); ok {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	l, ok := node.(yaml.List)
	if l == nil {
		// Looks like we don't get NodeNotFound from Child?
		return nil, nil
	}
	if !ok {
		return nil, fmt.Errorf("Expected yaml.List, got %T\n", node)
	}
	ret := make([]map[string]string, l.Len())
	for i, n := range l {
		ret[i] = make(map[string]string)
		m, ok := n.(yaml.Map)
		if !ok {
			return nil, fmt.Errorf("Expected yaml.Map, got %T\n", node)
		}
		for name, n2 := range m {
			s, ok := n2.(yaml.Scalar)
			if !ok {
				return nil, fmt.Errorf("Expected yaml.Scalar, got %T\n", node)
			}
			ret[i][name] = string(s)
		}
	}
	return ret, nil
}
