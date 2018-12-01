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
	c.Redis.MaxCon = 255
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
	// success case
	pool, err := readCfg()
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.Get()
	if !IsOk(conn) {
		t.Error("db check is not ok")
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
			ok, err := v.item.delete(conn)
			if err != nil {
				t.Errorf("failed delete item, case=%v: %v", i, err)
			}
			if !ok {
				t.Error("item was not deleted")
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
		ok, err := item.delete(conn)
		if err != nil {
			t.Error("failed delete item")
		}
		if !ok {
			t.Error("item was not deleted")
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
		ok, err := item.delete(conn)
		if err != nil {
			t.Error("failed delete item")
		}
		if !ok {
			t.Error("item was not deleted")
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
	exists, err := item.Read(conn, cipherKey)
	if exists {
		t.Error("unexpected success read")
	}
	item.Key = key
	// success read
	exists, err = item.Read(conn, cipherKey)
	if !exists || (err != nil) {
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
	exists, err = item.Read(conn, cipherKey)
	if !exists || (err != nil) {
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

func TestItem_ReadConcurrent(t *testing.T) {
	const (
		times   = 128
		workers = 8
	)
	pool, err := readCfg()
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.Get()
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
	item := &Item{Content: "test", TTL: 60, Times: times}
	err = item.Save(conn, cipherKey)
	if err != nil {
		t.Fatal(err)
	}
	key, content := item.Key, item.Content
	ch := make(chan int)
	for i := 0; i < workers; i++ {
		go func(n int) {
			x := &Item{Key: key, Content: content}
			for j := 0; j < times; j++ {
				c := pool.Get()
				exists, err := x.Read(c, cipherKey)
				if err != nil {
					t.Errorf("unexpected error read, worker=%v: %v", n, err)
				}
				if exists {
					ch <- 1
				} else {
					ch <- 0
				}
				err = c.Close()
				if err != nil {
					t.Errorf("close connection errror, worker=%v, attempt=%v: %v", n, j, err)
				}
			}
		}(i)
	}
	s := 0
	for i := 0; i < workers*times; i++ {
		s += <-ch
	}
	close(ch)
	if s != times {
		t.Errorf("failed sum=%v", s)
	}
}

func BenchmarkItem_Save(b *testing.B) {
	pool, err := readCfg()
	if err != nil {
		b.Fatal(err)
	}
	conn := pool.Get()
	defer func() {
		err = conn.Close()
		if err != nil {
			b.Errorf("close connection errror: %v", err)
		}
		err = pool.Close()
		if err != nil {
			b.Errorf("close pool errror: %v", err)
		}
	}()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		item := Item{Content: "test", TTL: 10, Times: 1}
		err = item.Save(conn, cipherKey)
		if err != nil {
			b.Errorf("failed save: %v", err)
		}
		ok, err := item.delete(conn)
		if err != nil {
			b.Errorf("failed delete: %v", err)
		}
		if !ok {
			b.Error("item was not deleted")
		}
	}
}

func BenchmarkItem_Read(b *testing.B) {
	pool, err := readCfg()
	if err != nil {
		b.Fatal(err)
	}
	conn := pool.Get()
	defer func() {
		err = conn.Close()
		if err != nil {
			b.Errorf("close connection errror: %v", err)
		}
		err = pool.Close()
		if err != nil {
			b.Errorf("close pool errror: %v", err)
		}
	}()
	item := Item{Content: "test", TTL: 10, Times: 1000000}
	err = item.Save(conn, cipherKey)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		ok, err := item.delete(conn)
		if err != nil {
			b.Errorf("failed delete: %v", err)
		}
		if !ok {
			b.Error("item was not deleted")
		}
	}()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		item.eContent, item.hPassword = "", ""
		exists, err := item.Read(conn, cipherKey)
		if !exists || (err != nil) {
			b.Errorf("failed read: %v", err)
		}
	}
}
