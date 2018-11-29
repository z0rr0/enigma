package web

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/z0rr0/enigma/conf"
	"github.com/z0rr0/enigma/db"
)

const (
	testConfigName = "/tmp/config.example.json"
)

var (
	rgCheck = regexp.MustCompile(`href="http(s)?://.+/(?P<key>[0-9a-z]{128})"`)
)

type createData struct {
	Method string
	Params [4]string // content, ttl, times, password
	Code   int
	Err    bool
}

type readData struct {
	Item     *db.Item
	Method   string
	Password string
	Code     int
	Err      bool
}

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
		{http.StatusBadRequest, "Bad createData"},
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
		b = []byte{}
	}
}

func TestIndex(t *testing.T) {
	var body io.Reader
	cfg, err := conf.New(testConfigName)
	if err != nil {
		t.Fatal(err)
	}
	conn := cfg.Connection()
	defer func() {
		err := conn.Close()
		if err != nil {
			t.Errorf("failed close connection: %v", err)
		}
		err = cfg.Close()
		if err != nil {
			t.Errorf("close error: %v", err)
		}
	}()
	values := []createData{
		{"GET", [4]string{}, http.StatusOK, false},
		{"POST", [4]string{"test", "10", "1", ""}, http.StatusOK, false},
		{"POST", [4]string{"test", "10", "1", "abc"}, http.StatusOK, false},
		{"POST", [4]string{"", "10", "1", ""}, http.StatusBadRequest, true},
		{"POST", [4]string{"test", "", "1", ""}, http.StatusBadRequest, true},
		{"POST", [4]string{"test", "10", "", ""}, http.StatusBadRequest, true},
	}
	b := make([]byte, 512)
	for i, v := range values {
		w := httptest.NewRecorder()

		if v.Method == "POST" {
			params := url.Values{}
			params.Set("content", v.Params[0])
			params.Set("ttl", v.Params[1])
			params.Set("times", v.Params[2])
			body = strings.NewReader(params.Encode())
		} else {
			body = nil
		}
		r := httptest.NewRequest(v.Method, "/", body)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		code, err := Index(w, r, cfg)
		if v.Err {
			if err == nil {
				t.Errorf("expected error for case=%v", i)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			} else {
				if code != v.Code {
					t.Errorf("failed case=%v code=%v", i, code)
				}
				if (code == http.StatusOK) && (v.Method == "POST") {
					// success result, expected key URL
					resp := w.Result()
					_, err = resp.Body.Read(b)
					if err != nil {
						t.Errorf("failed read body for case=%v", i)
					} else {
						finds := rgCheck.FindStringSubmatch(string(b))
						if l := len(finds); l != 3 {
							t.Errorf("failed result check lenght: %v", l)
						} else {
							key := finds[2]
							err = db.Delete(key, conn)
							if err != nil {
								t.Errorf("failed delete item case=%v: %v", i, err)
							}
						}
					}
				}
			}
		}
	}
}

func TestRead(t *testing.T) {
	var body io.Reader
	cfg, err := conf.New(testConfigName)
	if err != nil {
		t.Fatal(err)
	}
	cipherKey, err := hex.DecodeString(cfg.Key)
	if err != nil {
		t.Fatal(err)
	}
	conn := cfg.Connection()
	defer func() {
		err := conn.Close()
		if err != nil {
			t.Errorf("failed close connection: %v", err)
		}
		err = cfg.Close()
		if err != nil {
			t.Errorf("close error: %v", err)
		}
	}()
	values := []readData{
		{
			&db.Item{Content: "Test-Item-", TTL: 30, Times: 1, Password: "abc"},
			"POST", "abc", http.StatusOK, false,
		},
		{
			&db.Item{Content: "Test-Item-", TTL: 30, Times: 1, Password: ""},
			"POST", "", http.StatusOK, false,
		},
		{
			&db.Item{Content: "Test-Item-", TTL: 30, Times: 1, Password: "abc"},
			"POST", "bad", http.StatusBadRequest, false,
		},
		{
			&db.Item{Content: "Test-Item-", TTL: 30, Times: 1, Password: "abc"},
			"POST", "", http.StatusBadRequest, false,
		},
		{
			&db.Item{Content: "Test-Item-", TTL: 30, Times: 1},
			"GET", "", http.StatusOK, false,
		},
		{
			&db.Item{Content: "Test-Item-", TTL: 30, Times: 2, Password: "abc"},
			"POST", "abc", http.StatusOK, false,
		},
		{
			&db.Item{Content: "Test-Item-", TTL: 30, Times: 2, Password: ""},
			"POST", "", http.StatusOK, false,
		},
	}
	b := make([]byte, 512)
	for i, v := range values {
		w := httptest.NewRecorder()

		path := "/"
		if v.Item != nil {
			v.Item.Content = fmt.Sprintf("%v%v", v.Item.Content, i)
			err = v.Item.Save(conn, cipherKey)
			if err != nil {
				t.Errorf("failed save case=%v: %v", i, err)
				continue
			}
			path += v.Item.Key
		}
		if v.Method == "POST" {
			params := url.Values{}
			params.Set("password", v.Password)
			body = strings.NewReader(params.Encode())
		} else {
			body = nil
		}
		r := httptest.NewRequest(v.Method, path, body)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		code, err := Read(w, r, cfg)
		if v.Err {
			if err == nil {
				t.Errorf("expected error for case=%v", i)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			} else {
				if code != v.Code {
					t.Errorf("failed case=%v code=%v", i, code)
				}
				if (code == http.StatusOK) && (v.Method == "POST") && (v.Item != nil) && !v.Err {
					// success result, expected content
					resp := w.Result()
					_, err = resp.Body.Read(b)
					if err != nil {
						t.Errorf("failed read body for case=%v", i)
					} else {
						content := string(b)
						if j := strings.Index(content, v.Item.Content); j < 0 {
							t.Errorf("not found content, case=%v: %v", i, content)
						}
					}
					if v.Item.Times > 1 {
						err = db.Delete(v.Item.Key, conn)
						if err != nil {
							t.Errorf("failed delete item case=%v: %v", i, err)
						}
					} else {
						ok, err := v.Item.Exists(conn)
						if err != nil {
							t.Errorf("check exist error case=%v: %v", i, err)
						}
						if ok {
							t.Errorf("item unexpectedly exists, case=%v", i)
						}
					}
				} else if v.Item != nil {
					err = db.Delete(v.Item.Key, conn)
					if err != nil {
						t.Errorf("failed delete item case=%v: %v", i, err)
					}
				}
			}
		}
	}
}

func BenchmarkIndex(b *testing.B) {
	cfg, err := conf.New(testConfigName)
	if err != nil {
		b.Fatal(err)
	}
	conn := cfg.Connection()
	defer func() {
		err := conn.Close()
		if err != nil {
			b.Errorf("failed close connection: %v", err)
		}
		err = cfg.Close()
		if err != nil {
			b.Errorf("close error: %v", err)
		}
	}()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		w := httptest.NewRecorder()
		params := url.Values{}
		params.Set("content", "test")
		params.Set("ttl", "30")
		params.Set("times", "1")
		params.Set("password", "abc")

		r := httptest.NewRequest("POST", "/", strings.NewReader(params.Encode()))
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		code, err := Index(w, r, cfg)
		if err != nil {
			b.Error(err)
		}
		if code != http.StatusOK {
			b.Errorf("faield code: %v", code)
		}
	}
}

func BenchmarkRead(b *testing.B) {
	const password = "abc"
	cfg, err := conf.New(testConfigName)
	if err != nil {
		b.Fatal(err)
	}
	cipherKey, err := hex.DecodeString(cfg.Key)
	if err != nil {
		b.Fatal(err)
	}
	conn := cfg.Connection()
	defer func() {
		err := conn.Close()
		if err != nil {
			b.Errorf("failed close connection: %v", err)
		}
		err = cfg.Close()
		if err != nil {
			b.Errorf("close error: %v", err)
		}
	}()
	item := &db.Item{Content: "test", TTL: 30, Times: 1000000, Password: password}
	defer func() {
		err = db.Delete(item.Key, conn)
		if err != nil {
			b.Errorf("failed delete item: %v", err)
		}
	}()
	err = item.Save(conn, cipherKey)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		params := url.Values{}
		params.Set("password", password)

		r := httptest.NewRequest("POST", "/"+item.Key, strings.NewReader(params.Encode()))
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		code, err := Read(w, r, cfg)
		if err != nil {
			b.Error(err)
		}
		if code != http.StatusOK {
			b.Errorf("faield code: %v", code)
		}
	}
}
