package localexec

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func TestResourceProvisioner_impl(t *testing.T) {
	var _ terraform.ResourceProvisioner = Provisioner()
}

func TestProvisioner(t *testing.T) {
	if err := Provisioner().(*schema.Provisioner).InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestResourceProvider_Apply(t *testing.T) {
	defer os.Remove("test_out")
	c := testConfig(t, map[string]interface{}{
		"command": "echo foo > test_out",
	})

	output := new(terraform.MockUIOutput)
	p := Provisioner()

	if err := p.Apply(output, nil, c); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the file
	raw, err := ioutil.ReadFile("test_out")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	actual := strings.TrimSpace(string(raw))
	expected := "foo"
	if actual != expected {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestResourceProvider_stop(t *testing.T) {
	c := testConfig(t, map[string]interface{}{
		// bash/zsh/ksh will exec a single command in the same process. This
		// makes certain there's a subprocess in the shell.
		"command": "sleep 30; sleep 30",
	})

	output := new(terraform.MockUIOutput)
	p := Provisioner()

	doneCh := make(chan struct{})
	startTime := time.Now()
	go func() {
		defer close(doneCh)
		// The functionality of p.Apply is tested in TestResourceProvider_Apply.
		// Because p.Apply is called in a goroutine, trying to t.Fatal() on its
		// result would be ignored or would cause a panic if the parent goroutine
		// has already completed.
		_ = p.Apply(output, nil, c)
	}()

	mustExceed := (50 * time.Millisecond)
	select {
	case <-doneCh:
		t.Fatalf("expected to finish sometime after %s finished in %s", mustExceed, time.Since(startTime))
	case <-time.After(mustExceed):
		t.Logf("correctly took longer than %s", mustExceed)
	}

	// Stop it
	stopTime := time.Now()
	p.Stop()

	maxTempl := "expected to finish under %s, finished in %s"
	finishWithin := (2 * time.Second)
	select {
	case <-doneCh:
		t.Logf(maxTempl, finishWithin, time.Since(stopTime))
	case <-time.After(finishWithin):
		t.Fatalf(maxTempl, finishWithin, time.Since(stopTime))
	}
}

func TestResourceProvider_Validate_good(t *testing.T) {
	c := testConfig(t, map[string]interface{}{
		"command": "echo foo",
	})

	warn, errs := Provisioner().Validate(c)
	if len(warn) > 0 {
		t.Fatalf("Warnings: %v", warn)
	}
	if len(errs) > 0 {
		t.Fatalf("Errors: %v", errs)
	}
}

func TestResourceProvider_Validate_missing(t *testing.T) {
	c := testConfig(t, map[string]interface{}{})

	warn, errs := Provisioner().Validate(c)
	if len(warn) > 0 {
		t.Fatalf("Warnings: %v", warn)
	}
	if len(errs) == 0 {
		t.Fatalf("Should have errors")
	}
}

func testConfig(t *testing.T, c map[string]interface{}) *terraform.ResourceConfig {
	r, err := config.NewRawConfig(c)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}

	return terraform.NewResourceConfig(r)
}
