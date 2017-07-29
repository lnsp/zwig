package appengine

import (
	"net/http"
	"zwig/api"
	"zwig/web"
)

var (
	colors = []string{"blue", "red", "orange", "green"}
)

func init() {
	apiHandler := api.New()
	webHandler := web.New()
	http.Handle("/api/", apiHandler)
	http.Handle("/", webHandler)
}
