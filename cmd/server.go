package cmd

import (
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type TemplateParam struct {
	Title   string
	Body    string
	Host    string
	Reload  bool
	Mode    string
	DirMode bool
}

type IndexTemplateParam struct {
	Title  string
	Dir    string
	Files  []FileEntry
	Host   string
	Reload bool
	Mode   string
}

type FileEntry struct {
	Path string
	Name string
	Dir  string
}

type Server struct {
	host string
	port int
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

//go:embed template.html
var htmlTemplate string

//go:embed index_template.html
var indexHTMLTemplate string

const defaultPort = 3333

func (server *Server) Serve(param *Param) error {
	host := server.host
	port := defaultPort
	if server.port > 0 {
		port = server.port
	}

	// Check for directory mode: explicit directory argument
	if !param.useStdin && param.filename != "" {
		info, statErr := os.Stat(param.filename)
		if statErr == nil && info.IsDir() {
			return server.ServeDir(param)
		}
	}

	// Use a empty filename for stdin
	filename := ""
	if !param.useStdin {
		var err error
		filename, err = targetFile(param.filename)
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(filename)

	r := http.NewServeMux()
	r.Handle("/", wrapHandler(handler(filename, param, http.FileServer(http.Dir(dir)))))
	r.Handle("/__/md", wrapHandler(mdHandler(filename, param)))

	watcher, err := createWatcher(dir)
	if err != nil {
		return err
	}
	r.Handle("/ws", wsHandler(watcher))

	port, err = getPort(host, port)
	if err != nil {
		return err
	}

	address := fmt.Sprintf("%s:%d", host, port)

	logInfo("Accepting connections at http://%s/\n", address)

	if param.autoOpen {
		logInfo("Open http://%s/ on your browser\n", address)
		go openBrowser(fmt.Sprintf("http://%s/", address))
	}

	err = http.ListenAndServe(address, r)
	if err != nil {
		return err
	}

	return nil
}

func handler(filename string, param *Param, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if !strings.HasSuffix(r.URL.Path, ".md") && r.URL.Path != "/" {
			h.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		tmpl, err := template.New("HTML Template").Parse(htmlTemplate)
		if err != nil {
			logInfo("Warn: %v", err)
			http.NotFound(w, r)
			return
		}

		var markdown string
		if param.useStdin && param.stdinContent != "" && filename == "" {
			markdown = param.stdinContent
		} else {
			markdown, err = slurp(filename)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		html, err := toHTML(markdown, param)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		title := getTitle(filename)
		modeString := getModeString(param.forceLightMode, param.forceDarkMode)

		tparam := TemplateParam{Title: title, Body: html, Host: r.Host, Reload: param.reload, Mode: modeString}
		tmpl.Execute(w, tparam)
	})
}

func mdResponse(w http.ResponseWriter, filename string, param *Param) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var markdown string
	var err error
	if param.useStdin && param.stdinContent != "" && filename == "" {
		markdown = param.stdinContent
	} else {
		markdown, err = slurp(filename)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}

	html, err := toHTML(markdown, param)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprintf(w, "%s", html)

}

func mdHandler(filename string, param *Param) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathParam := r.URL.Query().Get("path")
		if pathParam != "" {
			mdResponse(w, pathParam, param)
		} else {
			mdResponse(w, filename, param)
		}
	})
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func wrapHandler(wrappedHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := NewLoggingResponseWriter(w)
		wrappedHandler.ServeHTTP(lrw, r)

		statusCode := lrw.statusCode
		logInfo("%s [%d] %s", r.Method, statusCode, r.URL)
	})
}

func getTitle(filename string) string {
	return filepath.Base(filename)
}

func getModeString(lightMode, darkMode bool) string {
	if lightMode {
		return "light"
	} else if darkMode {
		return "dark"
	}
	return ""
}

func getPort(host string, port int) (int, error) {
	var err error
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		logInfo(err.Error())
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:0", host))
	}
	port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port, err
}

// ServeDir starts a server that lists all markdown files in a directory.
func (server *Server) ServeDir(param *Param) error {
	host := server.host
	port := defaultPort
	if server.port > 0 {
		port = server.port
	}

	baseDir, err := filepath.Abs(param.filename)
	if err != nil {
		return err
	}

	r := http.NewServeMux()
	r.Handle("/", wrapHandler(dirHandler(baseDir, param, http.FileServer(http.Dir(baseDir)))))
	r.Handle("/__/md", wrapHandler(dirMdHandler(baseDir, param)))

	watcher, err := createRecursiveWatcher(baseDir)
	if err != nil {
		return err
	}
	r.Handle("/ws", wsHandler(watcher))

	port, err = getPort(host, port)
	if err != nil {
		return err
	}

	address := fmt.Sprintf("%s:%d", host, port)

	logInfo("Accepting connections at http://%s/\n", address)
	logInfo("Serving markdown files from %s\n", baseDir)

	if param.autoOpen {
		logInfo("Open http://%s/ on your browser\n", address)
		go openBrowser(fmt.Sprintf("http://%s/", address))
	}

	err = http.ListenAndServe(address, r)
	if err != nil {
		return err
	}

	return nil
}

func dirHandler(baseDir string, param *Param, fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Index page
		if r.URL.Path == "/" {
			serveIndex(w, r, baseDir, param)
			return
		}

		// Markdown file preview
		if strings.HasSuffix(r.URL.Path, ".md") {
			relPath := r.URL.Path[1:] // strip leading /
			absPath, err := safePath(baseDir, relPath)
			if err != nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			tmpl, err := template.New("HTML Template").Parse(htmlTemplate)
			if err != nil {
				logInfo("Warn: %v", err)
				http.NotFound(w, r)
				return
			}

			title := filepath.Base(relPath)
			modeString := getModeString(param.forceLightMode, param.forceDarkMode)
			tparam := TemplateParam{
				Title:   title,
				Host:    r.Host,
				Reload:  param.reload,
				Mode:    modeString,
				DirMode: true,
			}
			tmpl.Execute(w, tparam)
			return
		}

		// Static files (images, etc.)
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, baseDir string, param *Param) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	mdFiles, err := findMarkdownFiles(baseDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var entries []FileEntry
	for _, f := range mdFiles {
		dir := filepath.Dir(f)
		if dir == "." {
			dir = ""
		}
		entries = append(entries, FileEntry{
			Path: f,
			Name: filepath.Base(f),
			Dir:  dir,
		})
	}

	tmpl, err := template.New("Index Template").Parse(indexHTMLTemplate)
	if err != nil {
		logInfo("Warn: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dirName := filepath.Base(baseDir)
	modeString := getModeString(param.forceLightMode, param.forceDarkMode)

	tparam := IndexTemplateParam{
		Title:  dirName + " - Markdown Files",
		Dir:    dirName,
		Files:  entries,
		Host:   r.Host,
		Reload: param.reload,
		Mode:   modeString,
	}
	tmpl.Execute(w, tparam)
}

func dirMdHandler(baseDir string, param *Param) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathParam := r.URL.Query().Get("path")
		if pathParam == "" {
			http.Error(w, "path parameter required", http.StatusBadRequest)
			return
		}

		absPath, err := safePath(baseDir, pathParam)
		if err != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		mdResponse(w, absPath, param)
	})
}