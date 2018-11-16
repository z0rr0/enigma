package conf

import "testing"

const (
	testConfigName = "/tmp/config.example.json"
)

func TestNew(t *testing.T) {
	if _, err := New("/bad_file_path.json"); err == nil {
		t.Error("unexpected behavior")
	}
	cfg, err := New(testConfigName)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr() == "" {
		t.Error("empty address")
	}
	err = cfg.Close()
	if err != nil {
		t.Errorf("close error: %v", err)
	}
}
