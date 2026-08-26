package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/httpapi"
	"github.com/benzhi/ancient-tree-pathogen/internal/selfcheck"
	"github.com/benzhi/ancient-tree-pathogen/internal/storage"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	addr := httpapi.Addr(fs)
	self := fs.Bool("selfcheck", false, "运行自检")
	fs.Parse(os.Args[1:])
	if *self {
		if e := selfcheck.Run(context.Background()); e != nil {
			log.Fatal(e)
		}
		return
	}
	db, e := storage.Open("ancient_tree.db")
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	app := application.New(db)
	server := &http.Server{Addr: httpapi.NormalizeAddr(*addr), Handler: httpapi.New(app).Handler()}
	go func() {
		fmt.Printf("古树病原鉴定工作台已启动：http://%s\n", server.Addr)
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	server.Shutdown(context.Background())
}
