// Command package creates deterministic portable archives and SHA-256 manifests.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type file struct {
	name, path string
	mode       int64
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	version := flag.String("version", "0.4.0", "Release version")
	target := flag.String("os", "linux", "Target OS")
	binary := flag.String("binary", "", "Compiled executable")
	flag.Parse()
	if *target != "linux" && *target != "windows" {
		return fmt.Errorf("unsupported target")
	}
	if *binary == "" {
		return fmt.Errorf("missing binary")
	}
	name := "erp"
	installer := "scripts/install-linux.sh"
	if *target == "windows" {
		name += ".exe"
		installer = "scripts/install-windows.ps1"
	}
	files := []file{{name, *binary, 0755}, {filepath.Base(installer), installer, 0755}, {"INICIO.md", "docs/INDEX.md", 0644}}
	err := filepath.WalkDir("docs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, file{filepath.ToSlash(path), path, 0644})
		}
		return nil
	})
	if err != nil {
		return err
	}
	files = append(files, file{"api/openapi.json", "api/openapi.json", 0644})
	licenses, err := collectLicenses(*target)
	if err != nil {
		return err
	}
	files = append(files, licenses...)
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	base := "raiz-erp-" + *version + "-" + *target + "-amd64"
	archive := filepath.Join("artifacts", base+".tar.gz")
	if *target == "windows" {
		archive = filepath.Join("artifacts", base+".zip")
	}
	output, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer output.Close()
	if *target == "windows" {
		z := zip.NewWriter(output)
		for _, f := range files {
			info, e := os.Stat(f.path)
			if e != nil {
				return e
			}
			h := &zip.FileHeader{Name: base + "/" + f.name, Method: zip.Deflate, Modified: time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)}
			h.SetMode(os.FileMode(f.mode))
			h.UncompressedSize64 = uint64(info.Size())
			writer, e := z.CreateHeader(h)
			if e != nil {
				return e
			}
			if e = copyFile(writer, f.path); e != nil {
				return e
			}
		}
		if err = z.Close(); err != nil {
			return err
		}
	} else {
		gz := gzip.NewWriter(output)
		tarWriter := tar.NewWriter(gz)
		for _, f := range files {
			info, e := os.Stat(f.path)
			if e != nil {
				return e
			}
			h := &tar.Header{Name: base + "/" + f.name, Size: info.Size(), Mode: f.mode, Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR}
			if len(h.Name) > 250 {
				h.Format = tar.FormatPAX
			}
			if e = tarWriter.WriteHeader(h); e != nil {
				return e
			}
			if e = copyFile(tarWriter, f.path); e != nil {
				return e
			}
		}
		if err = tarWriter.Close(); err != nil {
			return err
		}
		if err = gz.Close(); err != nil {
			return err
		}
	}
	if err = output.Close(); err != nil {
		return err
	}
	all, _ := filepath.Glob("artifacts/raiz-erp-*")
	sort.Strings(all)
	var sums strings.Builder
	for _, path := range all {
		if !strings.HasSuffix(path, ".zip") && !strings.HasSuffix(path, ".tar.gz") {
			continue
		}
		input, e := os.Open(path)
		if e != nil {
			return e
		}
		hash := sha256.New()
		_, e = io.Copy(hash, input)
		input.Close()
		if e != nil {
			return e
		}
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(hash.Sum(nil)), filepath.Base(path))
	}
	if err = os.WriteFile("artifacts/SHA256SUMS", []byte(sums.String()), 0644); err != nil {
		return err
	}
	fmt.Println(archive)
	return nil
}
func copyFile(destination io.Writer, path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = io.Copy(destination, f)
	return e
}
func collectLicenses(target string) ([]file, error) {
	cmd := exec.Command("go", "list", "-deps", "-json", "./cmd/erp")
	cmd.Env = append(os.Environ(), "GOOS="+target, "GOARCH=amd64", "CGO_ENABLED=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("resolver licencias: %w: %s", err, stderr.String())
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var files []file
	seen := map[string]bool{}
	for {
		var pkg struct {
			Module *struct {
				Path, Version, Dir string
				Main               bool
			}
		}
		err = decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		module := pkg.Module
		if module == nil || module.Main || module.Dir == "" || seen[module.Path] {
			continue
		}
		seen[module.Path] = true
		entries, err := os.ReadDir(module.Dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			name := strings.ToUpper(entry.Name())
			if !entry.IsDir() && (strings.HasPrefix(name, "LICENSE") || strings.HasPrefix(name, "COPYING") || strings.HasPrefix(name, "COPYRIGHT") || strings.HasPrefix(name, "NOTICE")) {
				files = append(files, file{"licenses/go/" + strings.ReplaceAll(module.Path, "/", "_") + "-" + module.Version + "/" + entry.Name(), filepath.Join(module.Dir, entry.Name()), 0644})
			}
		}
	}
	for _, name := range []string{"react", "react-dom", "scheduler", "lucide-react", "jsbarcode", "fflate"} {
		license := "LICENSE"
		if name == "jsbarcode" {
			license = "MIT-LICENSE.txt"
		}
		path := filepath.Join("web/node_modules", name, license)
		if _, err = os.Stat(path); err != nil {
			return nil, err
		}
		files = append(files, file{"licenses/web/" + name + "-LICENSE", path, 0644})
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return nil, err
	}
	goLicense := filepath.Join(strings.TrimSpace(string(goroot)), "LICENSE")
	if _, e := os.Stat(goLicense); e != nil {
		// Distribution-packaged Go can keep its license outside GOROOT.
		goLicense = "/usr/share/licenses/go/LICENSE"
		if _, err = os.Stat(goLicense); err != nil {
			return nil, fmt.Errorf("no se encontró la licencia de Go en GOROOT ni en la distribución: %w", err)
		}
	}
	files = append(files, file{"licenses/Go-LICENSE", goLicense, 0644})
	return files, nil
}
