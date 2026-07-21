package env_test

import (
	"testing"

	"github.com/servicemesh/reference-app/internal/env"
)

func TestGet_DefaultAndSet(t *testing.T) {
	key := "SIM_POC_TEST_GET_XYZ"
	t.Setenv(key, "")
	// empty env → default (os.Getenv returns "" for unset; Setenv "" may still be set)
	// use unique unset key
	key2 := "SIM_POC_TEST_GET_UNSET_998877"
	if env.Get(key2, "def") != "def" {
		t.Fatal("default")
	}
	t.Setenv(key2, "val")
	if env.Get(key2, "def") != "val" {
		t.Fatal("set")
	}
}

func TestGetInt(t *testing.T) {
	key := "SIM_POC_TEST_INT_998877"
	if env.GetInt(key, 7) != 7 {
		t.Fatal("default")
	}
	t.Setenv(key, "42")
	if env.GetInt(key, 7) != 42 {
		t.Fatal("parse")
	}
	t.Setenv(key, "nope")
	if env.GetInt(key, 7) != 7 {
		t.Fatal("invalid → default")
	}
}
