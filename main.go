package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/lnsp/dodel/api"
	"github.com/lnsp/dodel/models"
	"github.com/lnsp/dodel/web"
)

var (
	port     = flag.String("port", "8080", "Public access port")
	dbfile   = flag.String("file", "db.json", "JSON database source")
	override = flag.Bool("override", false, "Override existing database")
	debug    = flag.Bool("debug", false, "Show debug messages")
)

var (
	colors   = []string{"blue", "red", "orange", "green"}
	database *models.JSONDatabase
)

func main() {
	flag.Parse()
	database = models.NewJSONDatabase(*dbfile, *override, *debug)
	apiHandler := api.New(database, *debug)
	webHandler := web.New(database, *debug)
	if *debug {
		log.Println("main: correctly setup handlers")
	}
	signals := make(chan os.Signal, 1)
	go func() {
		log.Println("main: waiting for cancelling signal")
		<-signals
		err := database.Save()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("main: successfully saved database")
		os.Exit(0)
	}()
	signal.Notify(signals, os.Interrupt)
	log.Println("main: subscribed to interrupt signal")
	if err := database.Load(); err != nil {
		log.Fatalln(err)
	}
	log.Println("main: successfully loaded database")
	http.Handle("/api/", apiHandler)
	http.Handle("/", webHandler)
	log.Println("main: listening for new connections")
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		fmt.Println(err)
	}
}
