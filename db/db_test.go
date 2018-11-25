package db

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gomodule/redigo/redis"
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
	jsonData, err := ioutil.ReadFile(testConfigName)
	if err != nil {
		t.Fatal(err)
	}
	c := &testCfg{}
	err = json.Unmarshal(jsonData, c)
	if err != nil {
		t.Fatal(err)
	}
	c.Redis.Db = testDbIndex
	c.Redis.Timeout = -1
	_, err = GetDbPool(c.Redis)
	if err == nil {
		t.Errorf("expected error")
	}
	c.Redis.Timeout = 10
	c.Redis.IndleCon = 0
	_, err = GetDbPool(c.Redis)
	if err == nil {
		t.Errorf("expected error")
	}
	c.Redis.IndleCon = 1
	c.Redis.Db = -1
	_, err = GetDbPool(c.Redis)
	if err == nil {
		t.Errorf("expected error")
	}

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
	const (
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

func TestItem_GetURL(t *testing.T) {
	key := "abc"
	uri := "http://example.com"

	r := httptest.NewRequest("POST", uri, nil)
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	item := &Item{Content: "test", TTL: 60, Times: 1, Key: key}

	expected := fmt.Sprintf("%v/%v", uri, key)
	if u := item.GetURL(r, false); u.String() != expected {
		t.Error("failed non-secure check", u.String())
	}

	uri = "https://example.com"
	expected = fmt.Sprintf("%v/%v", uri, key)
	if u := item.GetURL(r, true); u.String() != expected {
		t.Error("failed secure check", u.String())
	}
}

func TestItem_Save(t *testing.T) {
	pool, err := readCfg()
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = conn.Close()
		if err != nil {
			t.Errorf("close connection errror: %v", err)
		}
		err = pool.Close()
		if err != nil {
			t.Errorf("close pool errror: %v", err)
		}
	}()

	cases := []struct {
		item *Item
		skey []byte
		ok   bool
	}{
		{item: &Item{Content: "test", TTL: 60, Times: 1}, ok: false},
		{item: &Item{Content: "test", TTL: 60, Times: 1}, skey: cipherKey, ok: true},
		{item: &Item{Content: "test", TTL: 60, Times: 1, Password: "abc"}, skey: cipherKey, ok: true},
	}
	for i, v := range cases {
		err = v.item.Save(conn, v.skey)
		if v.ok {
			if err != nil {
				t.Errorf("unexpected error case=%v: %v", i, err)
			}
			err = v.item.delete(conn)
			if err != nil {
				t.Errorf("failed delete item, case=%v: %v", i, err)
			}
		} else {
			if err == nil {
				t.Errorf("expected error case=%v", i)
			}
		}
	}
}

func TestItem_Exists(t *testing.T) {
	pool, err := readCfg()
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = conn.Close()
		if err != nil {
			t.Errorf("close connection errror: %v", err)
		}
		err = pool.Close()
		if err != nil {
			t.Errorf("close pool errror: %v", err)
		}
	}()
	item := &Item{Content: "test", TTL: 60, Times: 1}
	err = item.Save(conn, cipherKey)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = item.delete(conn)
		if err != nil {
			t.Error("failed delete item")
		}
	}()

	exists, err := item.Exists(conn)
	if err != nil {
		t.Error(err)
	}
	if !exists {
		t.Error("item does not exist")
	}
	key := item.Key
	item.Key = "abc"

	exists, err = item.Exists(conn)
	if err != nil {
		t.Error(err)
	}
	if exists {
		t.Error("item exists")
	}
	item.Key = key
}

func TestItem_CheckPassword(t *testing.T) {
	const password = "abc"
	pool, err := readCfg()
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = conn.Close()
		if err != nil {
			t.Errorf("close connection errror: %v", err)
		}
		err = pool.Close()
		if err != nil {
			t.Errorf("close pool errror: %v", err)
		}
	}()
	item := &Item{Content: "test", TTL: 60, Times: 1, Password: password}
	err = item.Save(conn, cipherKey)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = item.delete(conn)
		if err != nil {
			t.Error("failed delete item")
		}
	}()
	ok, err := item.CheckPassword(conn)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("failed password check")
	}
	item.Password, item.hPassword = "bad", ""
	ok, err = item.CheckPassword(conn)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("unexpected success password check")
	}
}

func TestItem_Read(t *testing.T) {
	pool, err := readCfg()
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = conn.Close()
		if err != nil {
			t.Errorf("close connection errror: %v", err)
		}
		err = pool.Close()
		if err != nil {
			t.Errorf("close pool errror: %v", err)
		}
	}()
	item := &Item{Content: "test", TTL: 60, Times: 2}
	err = item.Save(conn, cipherKey)
	if err != nil {
		t.Fatal(err)
	}
	key := item.Key
	// read with failed key
	item.Key = "abc"
	err = item.Read(conn, cipherKey)
	if err == nil {
		t.Error("unexpected success read")
	}
	item.Key = key
	// success read
	err = item.Read(conn, cipherKey)
	if err != nil {
		t.Errorf("failed read; %v", err)
	}
	if times := item.Times; times != 1 {
		t.Errorf("invalid times value: %v", times)
	}
	ok, err := item.Exists(conn)
	if err != nil {
		t.Error("failed check existing")
	}
	if !ok {
		t.Errorf("item doesn't exist")
	}
	// read and delete
	err = item.Read(conn, cipherKey)
	if err != nil {
		t.Errorf("failed read; %v", err)
	}
	if times := item.Times; times != 0 {
		t.Errorf("invalid times value: %v", times)
	}
	ok, err = item.Exists(conn)
	if err != nil {
		t.Error("failed check existing")
	}
	if ok {
		t.Errorf("item still exist")
	}
}
