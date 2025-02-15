package shared

import (
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"

	"github.com/magicpantry/infra/gen/proto"
)

func ReadConfigInOtherManifest(key, manifestPath string) (*proto.ConfigItem, bool) {
	mf := ReadManifestAtPath(manifestPath)
	for _, item := range mf.Config.Items {
		if item.Name == key {
			return item, true
		}
	}
	return nil, false
}

func ReadRoot() *proto.Root {
	return ReadRootAtPath(findRootDir() + "/root.textproto")
}

func ReadRootAtPath(path string) *proto.Root {
	bs, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(path + ": " + err.Error())
	}

	var root proto.Root
	if err := prototext.Unmarshal(bs, &root); err != nil {
		log.Fatal(path + ": " + err.Error())
	}

	return &root
}

func ReadManifestAtPath(path string) *proto.Manifest {
	bs, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(path + ": " + err.Error())
	}

	var manifest proto.Manifest
	if err := prototext.Unmarshal(bs, &manifest); err != nil {
		log.Fatal(path + ": " + err.Error())
	}

	return &manifest
}

func ReadAllManifests() map[string]*proto.Manifest {
	found := map[string]*proto.Manifest{}

	rd := findRootDir()
	root := ReadRootAtPath(rd + "/root.textproto")

	enabled := map[string]any{}
	for _, s := range root.Services {
		enabled["/"+s] = struct{}{}
	}

	if err := filepath.WalkDir(rd, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		parts := strings.Split(path, "/")
		base := strings.Split(strings.Join(parts[:len(parts)-1], "/"), rd)[1]
		if _, ok := enabled[base]; !ok {
			return nil
		}
		if info.Name() == "manifest.textproto" {
			parts := strings.Split(path, "/")
			found[strings.Join(parts[:len(parts)-1], "/")] = ReadManifestAtPath(path)
		}

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	return found
}

func ReadManifest() map[string]*proto.Manifest {
	wd := currentDir()
	rd := findRootDir()

	for wd != rd {
		if has(filesInDir(wd), "manifest.textproto") {
			return map[string]*proto.Manifest{wd: ReadManifestAtPath(wd + "/manifest.textproto")}
		}

		parts := strings.Split(wd, "/")
		wd = strings.Join(parts[:len(parts)-1], "/")
	}

	found := map[string]*proto.Manifest{}

	if err := filepath.WalkDir(rd, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == "manifest.textproto" {
			parts := strings.Split(path, "/")
			found[strings.Join(parts[:len(parts)-1], "/")] = ReadManifestAtPath(path)
		}

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	return found
}

func has(xs []string, x string) bool {
	for _, c := range xs {
		if c != x {
			continue
		}
		return true
	}
	return false
}

func filesInDir(dir string) []string {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	var fs []string
	for _, fi := range fis {
		fs = append(fs, fi.Name())
	}
	return fs
}
