package main

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	SPACE byte = 32
	NUL   byte = 0
)

type Object struct {
	obj_type string
	size     string
	location string
	name     string
	content  []byte
}

type TreeEntry struct {
	mode string
	name string
	hash string
}

type Commit struct {
	tree    string
	parents []string
}

func getObjectsData(s string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	is_hex, err := regexp.MatchString("^[a-fA-F0-9]+$", filepath.Base(s))
	if !d.IsDir() && is_hex {
		println(s)
		var obj *Object = newObject(s)
		fmt.Printf("{type: \"%s\", size: \"%s\", content: %s}\n", obj.obj_type, obj.size, obj.toJson())
	}
	return nil
}

// Given a byte find the first byte in a data slice that equals the match_byte, returning the index.
// If no match is found, returns -1
func findFirstMatch(match_byte byte, start_index int, data *[]byte) int {
	for i, this_byte := range (*data)[start_index:] {
		if this_byte == match_byte {
			return start_index + i
		}
	}
	return -1
}

func getType(data *[]byte) (string, int) {
	first_space_index := findFirstMatch(SPACE, 0, data)
	obj_type := string((*data)[0:first_space_index])
	return strings.TrimSpace(obj_type), first_space_index
}

// second return value is the start of the object's content
func getSize(first_space_index int, data *[]byte) (string, int) {
	first_nul_index := findFirstMatch(NUL, first_space_index+1, data)
	obj_size := string((*data)[first_space_index:first_nul_index])
	return strings.TrimSpace(obj_size), first_nul_index + 1
}

func parseTree(obj *Object) []TreeEntry {
	var entries []TreeEntry
	entry_item, start, stop := 1, 0, 6
	mode, name, hash := "", "", ""
	for stop <= len(obj.content) {
		switch entry_item {
		// get the mode
		case 1:
			mode = strings.TrimSpace(string(obj.content[start:stop]))
			entry_item += 1
			start = stop
		// get the name (file or dir)
		case 2:
			i := start
			for obj.content[i] != NUL && i < len(obj.content)-1 {
				i += 1
			}
			name = strings.TrimSpace(string(obj.content[start:i]))
			entry_item += 1
			start = i + 1
			stop = start + 20
		// get the hash (object name)
		case 3:
			hash = strings.TrimSpace(hex.EncodeToString(obj.content[start:stop]))
			entry_item = 1
			start = stop
			stop = start + 6
			entries = append(entries, TreeEntry{mode, name, hash})
		}
	}
	return entries
}

func parseCommit(obj *Object) Commit {
	tree_hash := string(obj.content[5:45])
	rest_of_content := strings.Split(string(obj.content[46:]), "\n")
	var parents []string
	for _, line := range rest_of_content {
		if line[:6] == "parent" {
			parents = append(parents, line[7:47])
		} else {
			break
		}
	}
	return Commit{tree_hash, parents}
}

func newObject(object_path string) *Object {
	zlib_bytes, err := ioutil.ReadFile(object_path)
	if err != nil {
		panic(err)
	}
	// zlib expects an io.Reader object
	reader, err := zlib.NewReader(bytes.NewReader(zlib_bytes))
	if err != nil {
		panic(err)
	}
	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		panic(err)
	}
	data_ptr := &bytes
	obj_type, first_space_index := getType(data_ptr)
	size, content_start_index := getSize(first_space_index, data_ptr)
	return &Object{obj_type, size, object_path, filepath.Base(object_path), bytes[content_start_index:]}
}

func (obj *Object) toJson() string {
	switch obj.obj_type {
	case "tree":
		entries := parseTree(obj)
		output := "[\n"
		for i, entry := range entries {
			if i == len(entries)-1 {
				output += fmt.Sprintf("{mode: \"%s\", name: \"%s\", hash: \"%s\"}\n", entry.mode, entry.name, entry.hash)
			} else {
				output += fmt.Sprintf("{mode: \"%s\", name: \"%s\", hash: \"%s\"},\n", entry.mode, entry.name, entry.hash)
			}
		}
		return output + "]\n"
	case "commit":
		commit := parseCommit(obj)
		return fmt.Sprintf("{ parents: %s, tree: \"%s\"}", commit.parents, commit.tree)
	case "blob":
		return "I'm a blob"
	default:
		return fmt.Sprintf("I'm a %s\n", obj.obj_type)
	}
}

func main() {
	filepath.WalkDir(os.Args[1], getObjectsData)
}