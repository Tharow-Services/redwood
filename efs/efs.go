// Package efs is a handler to select between os and embedded file systems
package efs

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// embedded Filesystem For Included Settings
//go:embed builtin
//go:embed builtin/categories
//go:embed builtin/pages
var embedded embed.FS

const (

	// Prefix for selecting from embedded instead of os.
	Prefix = ">>"

	// Replacement for Prefix.
	Replacement = "builtin/"

	// number of times to replace Prefix with Replacement in FixEmbeddedPath.
	number = 1

	// errorPrefix the error prefix.
	errorPrefix = "embedded fs: %s"

	// filePermissions sets the file permissions used by os.WriteFile
	filePermissions fs.FileMode = 0666
)

// GetInternalFS warring do not use this function unless you know what you're doing
// panics with access violation if disableGetInternalFs is enabled
func GetInternalFS() *embed.FS {
	if disableGetInternalFs {
		panic("Embedded File System efs.go:GetInternalFS()->embed.FS: ACCESS VIOLATION TO EMBEDDED FILESYSTEM")
	}
	return &embedded
}

// disableGetInternalFs causes GetInternalFS to panic with access violation
var disableGetInternalFs = false

func LockInternalFS() { disableGetInternalFs = true }

// ReadDir like fs.ReadDir but selects between embedded fs and os
func ReadDir(path string) ([]fs.DirEntry, error) {
	if IsEmbed(path) {
		dir, err := embedded.ReadDir(strings.Replace(path, Prefix, Replacement, number))
		if err != nil {
			return nil, fmt.Errorf(errorPrefix, err)
		}
		return dir, nil
	} else {
		return os.ReadDir(path)
	}
}

// Open like fs.Open but selects between embedded fs and os
func Open(name string) (fs.File, error) {
	if IsEmbed(name) {
		file, err := embedded.Open(strings.Replace(name, Prefix, Replacement, number))
		if err != nil {
			return nil, fmt.Errorf(errorPrefix, err)
		}
		return file, nil
	} else {
		return os.Open(name)
	}
}

// ReadFile like fs.ReadFile but selects between embedded fs and os
func ReadFile(name string) ([]byte, error) {
	if IsEmbed(name) {
		con, err := embedded.ReadFile(strings.Replace(name, Prefix, Replacement, number))
		if err != nil {
			return nil, fmt.Errorf(errorPrefix, err)
		}
		return con, nil
	} else {
		return os.ReadFile(name)
	}
}

// WriteFile reads a file with ReadFile, and writes it to named file with os.WriteFile, with specified filePermissions
func WriteFile(path, name string) error {
	con, err := ReadFile(path)
	if err != nil {
		return fmt.Errorf("efs.WriteFile(%s, %s): %s", path, name, err)
	}
	err = os.WriteFile(name, con, filePermissions)
	if err != nil {
		return fmt.Errorf("efs.WriteFile(%s, %s): %s", path, name, err)
	}
	return nil
}

// Glob like fs.Glob but selects between embedded fs and os
func Glob(pattern string) ([]string, error) {
	var isEmbed = false
	if IsEmbed(pattern) {
		pattern = FromEmbed(pattern)
		isEmbed = true
	}
	if !isEmbed {
		return filepath.Glob(pattern)
	}
	var ret []string
	re, err := fs.Glob(embedded, pattern)
	if err != nil {
		log.Printf("efs.Glob(%s):fs.Glob(nil, %s)", pattern, err)
	}
	for _, s := range re {
		re = append(re, ToEmbed(s))
	}
	return ret, nil

}

// ToEmbed to embedded format
func ToEmbed(s string) string {
	if IsEmbed(s) {
		return s
	}
	return strings.Replace(s, Replacement, Prefix, number)
}

// FromEmbed to normal path format
func FromEmbed(s string) string {
	if !IsEmbed(s) {
		return s
	}
	return strings.Replace(s, Prefix, Replacement, number)
}

func IsEmbed(s string) bool {
	return strings.HasPrefix(s, Prefix)
}

// Join like filepath.Join but if the first element has Prefix then it joins path instead with path.Join
func Join(elem ...string) string {
	if !IsEmbed(elem[0]) {
		return filepath.Join(elem...)
	}
	var pre = []string{FromEmbed(elem[0])}
	elem[0] = ""
	return ToEmbed(path.Join(append(pre, elem...)...))
}

func Base(s string) string {
	if IsEmbed(s) {
		return ToEmbed(path.Base(FromEmbed(s)))
	}
	return filepath.Base(s)
}
