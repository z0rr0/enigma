package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/z0rr0/enigma/conf"
)

const (
	testConfigName = "/tmp/config.example.json"
)

func TestError(t *testing.T) {
	cfg, err := conf.New(testConfigName)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = cfg.Close()
		if err != nil {
			t.Errorf("close error: %v", err)
		}
	}()
	values := []struct {
		Code     int
		Expected string
	}{
		{http.StatusNotFound, "Page not found"},
		{http.StatusBadRequest, "Bad request"},
		{http.StatusInternalServerError, "it is an error"},
		{http.StatusMethodNotAllowed, "it is an error"},
	}
	b := make([]byte, 512)
	for i, v := range values {
		w := httptest.NewRecorder()
		code := Error(w, cfg, v.Code)
		if code != v.Code {
			t.Errorf("failed result for case=%v code: %v", i, code)
		} else {
			resp := w.Result()
			_, err = resp.Body.Read(b)
			if err != nil {
				t.Errorf("failed read body for case=%v", i)
			} else {
				strings.Contains(string(b), v.Expected)
			}
		}
	}
}
