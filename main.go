//go:generate go run -tags generate generate.go

package main

import (
	"fmt"
	"github.com/tylersammann/zipper/zippermerge"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	pdf "github.com/hhrutter/pdfcpu/pkg/pdfcpu"
	"github.com/tylersammann/zipper/static"
	"github.com/zserge/lorca"
)

func mergeFiles(fileName1, fileName2 string, rev1, rev2 bool) string {
	fmt.Printf("file1 %s reverse %v\n", fileName1, rev1)
	fmt.Printf("file2 %s reverse %v\n", fileName2, rev2)

	mergeFilename := fmt.Sprintf("%s_%s_%v.pdf",
		strings.Split(fileName1, ".")[0],
		strings.Split(fileName2, ".")[0],
		time.Now().Format("2006-01-02T15_04_05"))

	config := pdf.NewDefaultConfiguration()

	ctx1, err := pdf.ReadFile(fileName1, config)
	if err != nil {
		panic(err)
	}

	ctx2, err := pdf.ReadFile(fileName2, config)
	if err != nil {
		panic(err)
	}

	if ctx1.XRefTable.Version() < pdf.V15 {
		v, _ := pdf.PDFVersion("1.5")
		ctx1.XRefTable.RootVersion = &v
	}

	if ctx2.XRefTable.Version() < pdf.V15 {
		v, _ := pdf.PDFVersion("1.5")
		ctx2.XRefTable.RootVersion = &v
	}

	err = zippermerge.ZipperMergeXRefTables(ctx2, ctx1, rev2, rev1)
	if err != nil {
		panic(err)
	}

	dirName, fileName := filepath.Split(mergeFilename)
	ctx1.Write.DirName = dirName
	ctx1.Write.FileName = fileName

	err = pdf.Write(ctx1)
	if err != nil {
		panic(err)
	}

	return mergeFilename
}

func main() {
	// Start up the browser window
	ui, err := lorca.New("", "", 480, 380)
	if err != nil {
		log.Fatal(err)
	}
	defer ui.Close()

	ui.Bind("mergeFiles", mergeFiles)

	// Serve the static files
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	go http.Serve(ln, http.FileServer(static.FS))
	ui.Load(fmt.Sprintf("http://%s", ln.Addr()))

	// Wait until UI window is closed
	<-ui.Done()
}