package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/gabriel-vasile/mimetype"
	gonanoid "github.com/matoous/go-nanoid"

	"github.com/codenoid/file.io/storage"
)

func main() {
	box := rice.MustFindBox("web")
	fs := http.FileServer(box.HTTPBox())
	indexHTML, _ := box.Bytes("views/index.html")

	dsn := os.Getenv("DSN")
	if dsn == "" {
		dsn = "badger:./data"
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	strg, err := storage.Connect(dsn)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// runtime test
	if err := strg.Set("test", []byte("test"), 1*time.Second); err != nil {
		fmt.Println("runtime test failed:", err)
		os.Exit(1)
	}

	srv := &Server{FS: fs, Storage: strg, IndexHTML: indexHTML}
	fmt.Println("starting server on " + addr)
	fmt.Println(http.ListenAndServe(addr, http.HandlerFunc(srv.Index)))
}

type Server struct {
	FS        http.Handler
	Storage   storage.StorageHandler
	IndexHTML []byte
}

// Index for testing
func (srv *Server) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {

		// Serve static file
		if len(r.URL.Path) > 7 {
			if r.URL.Path[:7] == "/static" {
				srv.FS.ServeHTTP(w, r)
				return
			}
		}

		// show index.html
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			w.Write(srv.IndexHTML)
			return
		}

		// has file id
		fileID := strings.ReplaceAll(r.URL.Path, "/", "")
		if len(fileID) == 6 {

			// get file from database
			bytes, err := srv.Storage.Get(fileID)
			if err == nil {

				drop := false
				// download rate limiting, check if the quota are still sufficient
				quotaByte, err := srv.Storage.Get("mg-" + fileID)
				if err == nil {
					quota, err := strconv.Atoi(string(quotaByte))
					if err == nil {
						// fix this
						srv.Storage.Set("mg-"+fileID, []byte(strconv.Itoa(quota-1)), 0)
						if quota <= 1 {
							drop = true
						}
					}
				}

				// set Content-Disposition header if fn-<file-id> are exist
				filename, err := srv.Storage.Get("fn-" + fileID)
				if err == nil {
					w.Header().Set("Content-Disposition", "attachment; filename="+string(filename))
					if strings.Contains(string(filename), ".apk") {
						w.Header().Set("Content-Type", "application/vnd.android.package-archive")
					} else {
						w.Header().Set("Content-Type", mimetype.Detect(bytes).String())
					}
				}

				// write file []byte as response
				w.Write(bytes)

				if drop {
					// Delete file from storage
					srv.Storage.Del(fileID)
					srv.Storage.Del("fn-" + fileID)
					srv.Storage.Del("mg-" + fileID)
				}

				return
			}
			srv.Storage.Del("mg-" + fileID)
		}

		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": false, "error": 404, "message": "file not found"} `))
		return
	}

	if r.Method == "POST" {

		fileExp := 30 * time.Minute // in minute
		fileExpStr := r.URL.Query().Get("exp")
		if fileExpStr != "" {
			duration, err := time.ParseDuration(fileExpStr)
			if err != nil {
				w.Write([]byte(`{"success": false, "error": 402, "message": "A duration string is a possibly signed sequence of decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'. Valid time units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'. "} `))
				return
			}
			fileExp = duration
		}

		maxDownload := 1
		maxDownloadStr := r.URL.Query().Get("max")
		if maxDownloadStr != "" {
			maxInt, err := strconv.Atoi(maxDownloadStr)
			if err != nil {
				w.Write([]byte(`{"success": false, "error": 402, "message": "max download must be digit only"} `))
				return
			}
			maxDownload = maxInt
		}

		r.ParseMultipartForm(1000 << 20)
		// FormFile returns the first file for the given key `file`
		// it also returns the FileHeader so we can get the Filename,
		// the Header and the size of the file
		file, fileHeader, err := r.FormFile("file")
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`{"success": false, "error": 402, "message": "%v"}`, err.Error())))
			return
		}
		defer file.Close()

		// read all of the contents of our uploaded file into a
		// byte array, fix this
		fileBytes, err := ioutil.ReadAll(file)
		if err == nil {
			// generate random but short unique based on nanoid
			// the payload are AiUeO69 with length 6
			id, err := gonanoid.Generate("AiUeO69", 6)
			if err == nil {
				// set file content with id as key
				err = srv.Storage.Set(id, fileBytes, fileExp)
				if err == nil {
					// set file max get / read
					srv.Storage.Set("mg-"+id, []byte(strconv.Itoa(maxDownload)), (fileExp + 10))
					// set file name expiration with fn-<file-id> as key
					srv.Storage.Set("fn-"+id, []byte(fileHeader.Filename), (fileExp + 10))
					// setup json response
					data := map[string]interface{}{
						"success": true,
						"key":     id,
						"link":    "http://" + r.Host + "/" + id,
						"expiry":  fileExp.String(),
						"sec_exp": fileExp.Seconds(),
					}
					resp, _ := json.Marshal(data)
					w.Write(resp)
					return
				}
			}
		}
		w.Write([]byte(fmt.Sprintf(`{"success": false, "error": 402, "message": "%v"}`, err.Error())))
		return
	}
}
