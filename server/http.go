package server

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/Gigamons/Kokoro/handler"
	"github.com/Gigamons/common/logger"
	"github.com/gorilla/mux"
)

func middleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println("------------ERROR------------")
				fmt.Println(err)
				fmt.Println("---------ERROR TRACE---------")
				fmt.Println(string(debug.Stack()))
				fmt.Println("----------END ERROR----------")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func unknownWeb(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info(" %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func webFolder(w http.ResponseWriter, r *http.Request) {
	logger.Debug(r.URL.RawQuery)
	w.Write([]byte("not yet"))
}

// StartServer starts our HTTP Server.
func StartServer(host string, port int16) {
	r := mux.NewRouter()
	r.Use(middleWare)
	r.Use(unknownWeb)

	r.HandleFunc("/{avatar}", handler.GETAvatar)
	r.HandleFunc("/web/osu-search.php", handler.SearchDirect)
	r.HandleFunc("/web/osu-search-set.php", handler.GETDirectSet)
	r.HandleFunc("/web/{web}", webFolder)

	logger.Info(" Kokoro is listening on port %v\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%v", host, port), r))
}