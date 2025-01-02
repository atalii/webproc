package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-cmd/cmd"
)

//go:embed index.html
var indexTempl string

func main() {
	cmd := os.Args[1:]

	if len(cmd) == 0 {
		log.Fatal("specify child process as CLI args.")
	}

	log.Println("using:", cmd)

	run(cmd)
}

func run(args []string) {
	stdin, tx := io.Pipe()

	c := cmd.NewCmdOptions(cmd.Options{Streaming: true}, args[0], args[1:]...)
	c.StartWithStdin(stdin)
	serve(c, tx, args[0])
}

func streamer(name string, stream chan string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "can't convert to flusher", http.StatusInternalServerError)
		}

		w.Header().Add("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		for line := range stream {
			log.Printf("%s: %s", name, line)

			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}
}

func serve(cmd *cmd.Cmd, stdin io.Writer, cmd_name string) {
	index_t, err := template.New("index").Parse(indexTempl)
	index_buf := new(bytes.Buffer)
	if err := index_t.Execute(index_buf, map[string]any{
		"cmd_name": cmd_name,
	}); err != nil {
		log.Fatal("couldn't template index: ", err)
	}

	if err != nil {
		log.Fatal("couldn't build index template:", err)
	}

	http.Handle("/stdout", streamer("stdout", cmd.Stdout))
	http.Handle("/stderr", streamer("stderr", cmd.Stderr))

	http.HandleFunc("/stdin", func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(stdin, r.Body); err != nil {
			log.Println("couldn't write to stdin: ", err)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Write(index_buf.Bytes())
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
