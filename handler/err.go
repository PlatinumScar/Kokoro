package handler

import (
	"io/ioutil"
	"net/http"

	"github.com/Gigamons/common/logger"
)

func POSTosuerror(w http.ResponseWriter, r *http.Request) {
	logger.Debugln(r.URL.RawPath)
	r.ParseMultipartForm(0)
	b := BodyReader(r)
	logger.Debugln(b)
}

func BodyReader(r *http.Request) []byte {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Errorln(err)
		return nil
	}
	defer r.Body.Close()
	return b
}
