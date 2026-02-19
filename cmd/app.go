package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cli/safeexec"
)

func targetFile(filename string) (string, error) {
	var err error
	if filename == "" {
		filename = "."
	}
	info, err := os.Stat(filename)
	if err == nil && info.IsDir() {
		readme, err := findReadme(filename)
		if err != nil {
			return "", err
		}
		filename = readme
	}
	if err != nil {
		err = fmt.Errorf("%s is not found", filename)
	}
	return filename, err
}

func findReadme(dir string) (string, error) {
	files, _ := ioutil.ReadDir(dir)
	for _, f := range files {
		r := regexp.MustCompile(`(?i)^readme`)
		if r.MatchString(f.Name()) {
			return filepath.Join(dir, f.Name()), nil
		}
	}
	err := fmt.Errorf("README file is not found in %s/", dir)
	return "", err
}

func toHTML(markdown string, param *Param) (string, error) {
	mode := "gfm"
	if param.markdownMode {
		mode = "markdown"
	}
	sout, _, err := gh("api", "-X", "POST", "/markdown", "-f", fmt.Sprintf("text=%s", markdown), "-f", fmt.Sprintf("mode=%s", mode))
	if err != nil {
		return "", err
	}
	return sout.String(), nil
}

func slurp(fileName string) (string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, _ := ioutil.ReadAll(f)
	text := string(b)
	return text, nil
}

func gh(args ...string) (sout, eout bytes.Buffer, err error) {
	ghBin, err := safeexec.LookPath("gh")
	if err != nil {
		err = fmt.Errorf("could not find gh. Is it installed? error: %w", err)
		return
	}

	cmd := exec.Command(ghBin, args...)
	cmd.Stderr = &eout
	cmd.Stdout = &sout

	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("failed to run gh. error: %w, stderr: %s", err, eout.String())
		return
	}

	return
}

// findMarkdownFiles recursively finds all .md files in dir,
// returning paths relative to dir, sorted with README files first.
func findMarkdownFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories (but not the root)
			if strings.HasPrefix(info.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			// Skip node_modules, vendor
			if info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		iIsReadme := strings.HasPrefix(strings.ToLower(filepath.Base(files[i])), "readme")
		jIsReadme := strings.HasPrefix(strings.ToLower(filepath.Base(files[j])), "readme")
		if iIsReadme != jIsReadme {
			return iIsReadme
		}
		return files[i] < files[j]
	})

	return files, nil
}

// safePath validates that relPath doesn't escape baseDir.
// Returns the absolute path to the file.
func safePath(baseDir, relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute paths not allowed: %s", relPath)
	}
	abs := filepath.Join(baseDir, cleaned)
	// Ensure result is under baseDir
	if !strings.HasPrefix(abs+string(filepath.Separator), baseDir+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal detected: %s", relPath)
	}
	return abs, nil
}