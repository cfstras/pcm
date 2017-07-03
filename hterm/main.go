package hterm

import (
	"log"
	"net/http"

	"time"

	"fmt"
	"math/rand"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/skratchdot/open-golang/open"
)

func Run() {
	http.Handle("/",
		http.FileServer(
			&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: nil, Prefix: ""}))

	port := rand.Intn(65535-1024) + 1024
	addr := fmt.Sprint("127.0.0.1:", port)
	go startWeb(addr)
	time.Sleep(1 * time.Second)
	open.Start("http://" + addr + "/index.html")

}

func startWeb(addr string) {
	log.Fatal(http.ListenAndServe(addr, nil))
}
