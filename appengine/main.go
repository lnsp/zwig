package appengine

import (
	"net/http"

	"github.com/lnsp/zwig/web"

	"github.com/lnsp/zwig/api"
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
