// Copyright 2013 Julian Phillips.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package opgp

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/clearsign"
)

var KeyringFile = "keyring"

type UnknownKey struct {
	Key string
}

func (uk *UnknownKey) Error() string {
	return fmt.Sprintf("Unable to find a key with ID: %s", uk.Key)
}

type TooManyIdentities struct {
	Key   string
	Count int
	Max   int
}

func (tmi *TooManyIdentities) Error() string {
	return fmt.Sprintf("Key '%s' has %d identities, can only handle %d", tmi.Key, tmi.Count, tmi.Max)
}

type NoIdentities struct {
	Key string
}

func (ni *NoIdentities) Error() string {
	return fmt.Sprintf("Key '%s' has no identities", ni.Key)
}

func findKey(key string) (*openpgp.Entity, error) {
	keyId, err := strconv.ParseUint(key, 16, 64)
	if err != nil {
		log.Printf("Unable to parse key '%s': %s\n", key, err)
		return nil, &UnknownKey{key}
	}
	f, err := os.Open(KeyringFile)
	if err != nil {
		log.Printf("Failed to open keyring: %s\n", err)
		return nil, err
	}
	defer f.Close()
	el, err := openpgp.ReadKeyRing(f)
	if err != nil {
		log.Printf("Failed to read keyring: %s\n", err)
		return nil, err
	}
	for _, entity := range el {
		if entity.PrimaryKey.KeyId&0xFFFFFFFF == keyId {
			return entity, nil
		}
		for _, key := range entity.Subkeys {
			if key.PublicKey.KeyId&0xFFFFFFFF == keyId {
				return entity, nil
			}
		}
	}
	return nil, &UnknownKey{key}
}

func SignFile(filename, output, key string) error {
	entity, err := findKey(key)
	if err != nil {
		return err
	}

	in, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	return openpgp.ArmoredDetachSign(out, entity, in, nil)
}

func ExportKey(key, filename string) error {
	entity, err := findKey(key)
	if err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := armor.Encode(f, openpgp.PublicKeyType, nil)
	if err != nil {
		return err
	}

	err = entity.Serialize(w)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return nil
}

func GetSignerName(key string) (string, error) {
	entity, err := findKey(key)
	if err != nil {
		return "", err
	}

	if len(entity.Identities) > 1 {
		return "", &TooManyIdentities{key, len(entity.Identities), 1}
	}

	for name := range entity.Identities {
		return name, nil
	}

	return "", &NoIdentities{key}
}

func Clearsign(input io.Reader, output io.Writer, key string) error {
	entity, err := findKey(key)
	if err != nil {
		return err
	}

	w, err := clearsign.Encode(output, entity.PrivateKey, nil)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, input)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return nil
}

func ClearsignFile(filename, output, key string) error {
	in, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	return Clearsign(in, out, key)
}
