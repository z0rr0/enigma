package db

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	testConfigName = "/tmp/config.example.json"
	testDbIndex    = 1 // forced overwrite db index for tests
)

var (
	cipherKey = []byte{
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
)

type testCfg struct {
	Redis *Cfg `json:"redis"`
}

func readCfg() (*redis.Pool, error) {
	jsonData, err := ioutil.ReadFile(testConfigName)
	if err != nil {
		return nil, err
	}
	c := &testCfg{}
	err = json.Unmarshal(jsonData, c)
	if err != nil {
		return nil, err
	}
	c.Redis.Db = testDbIndex
	pool, err := GetDbPool(c.Redis)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func TestGetDbPool(t *testing.T) {
	pool, err := readCfg()
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Close()
	if err != nil {
		t.Errorf("close connection errror: %v", err)
	}
	err = pool.Close()
	if err != nil {
		t.Errorf("close pool errror: %v", err)
	}
}

func TestItem_New(t *testing.T) {
	var (
		maxTTL   = 300
		maxTimes = 10
	)
	cases := [][4]string{
		// content, ttl, times, ok
		{"test", "100", "1", "1"},
		{"test", "300", "1", "1"},
		{"test", "330", "1", "0"},
		{"test", "0", "1", "0"},
		{"test", "100", "0", "0"},
		{"test", "100", "11", "0"},
		{"test", "100", "10", "1"},
		{"", "100", "10", "0"},
		{"test", "bad", "10", "0"},
		{"test", "10", "bad", "0"},
		{"test", "", "10", "0"},
		{"test", "10", "", "0"},
		{"", "", "", "0"},
	}
	for i, v := range cases {
		values := url.Values{}
		values.Set("content", v[0])
		values.Set("ttl", v[1])
		values.Set("times", v[2])

		r := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		item, err := New(r, maxTTL, maxTimes)
		if v[3] == "1" {
			if err != nil {
				t.Errorf("unexpected error for case=%v: %v", i, err)
			} else {
				if item.Content != v[0] {
					t.Errorf("failed case=%v, content=%v", i, item.Content)
				}
			}
		} else {
			if err == nil {
				t.Errorf("expected error for case=%v", i)
			}
		}
	}
}

//func TestItem_Save(t *testing.T) {
//	pool, err := readCfg()
//	if err != nil {
//		t.Fatal(err)
//	}
//	conn := pool.Get()
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer func() {
//		err = conn.Close()
//		if err != nil {
//			t.Errorf("close connection errror: %v", err)
//		}
//		err = pool.Close()
//		if err != nil {
//			t.Errorf("close pool errror: %v", err)
//		}
//	}()
//}
