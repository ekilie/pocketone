package jsvm

import (
	"net/http"
	"net/http/httptest"
)

func newRec() *httptest.ResponseRecorder { return httptest.NewRecorder() }
func mkReq(url string) *http.Request     { return httptest.NewRequest("GET", url, nil) }
