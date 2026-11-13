package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != "127.0.0.1:8080" || c.DataDir != "data" || c.MaxUploadBytes != 64<<20 || c.SessionLifetime != 168*time.Hour {
		t.Fatalf("默认值异常: %+v", c)
	}
	if c.Layout.DBPath == "" || c.Layout.KeyPath == "" {
		t.Fatal("布局未计算")
	}
}

func TestLoadOverridesAndValidation(t *testing.T) {
	c, err := Load(env(map[string]string{"CNC_ADDR": "0.0.0.0:9000", "CNC_DATA_DIR": "/data", "CNC_MAX_UPLOAD_MB": "8", "CNC_SESSION_HOURS": "12"}))
	if err != nil || c.Addr != "0.0.0.0:9000" || c.MaxUploadBytes != 8<<20 || c.SessionLifetime != 12*time.Hour {
		t.Fatalf("覆盖失败: %+v %v", c, err)
	}
	for _, bad := range []map[string]string{
		{"CNC_MAX_UPLOAD_MB": "0"}, {"CNC_MAX_UPLOAD_MB": "abc"}, {"CNC_SESSION_HOURS": "0"}, {"CNC_SESSION_HOURS": "99999"},
	} {
		if _, err := Load(env(bad)); err == nil {
			t.Errorf("%v 应报错", bad)
		}
	}
}
