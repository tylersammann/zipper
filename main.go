//go:generate go run -tags generate generate.go

package main

import (
	"fmt"
	"github.com/tylersammann/zipper/static"
	"log"
	"net"
	"net/http"

	"github.com/zserge/lorca"
)

func setFile(fileId, jsonBytes string) {
	fmt.Printf("file id %s\n", fileId)
	fmt.Printf("file data %s\n", jsonBytes)
}

func main() {
	// Start up the browser window
	ui, err := lorca.New("", "", 480, 320)
	if err != nil {
		log.Fatal(err)
	}
	defer ui.Close()

	ui.Bind("setFile", setFile)

	// Serve the static files
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	go http.Serve(ln, http.FileServer(static.FS))
	ui.Load(fmt.Sprintf("http://%s", ln.Addr()))

	// Wait until UI window is closed
	<-ui.Done()
}