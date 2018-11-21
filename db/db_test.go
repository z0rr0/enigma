package db

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"io/ioutil"
	"testing"
)

const (
	testConfigName = "/tmp/config.example.json"
	testDbIndex    = 1
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
