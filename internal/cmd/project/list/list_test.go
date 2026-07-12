package list

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/ankitpokhrel/jira-cli/pkg/jira"
)

func TestOutputRawJSON(t *testing.T) {
	projects := []*jira.Project{
		{Key: "TEST", Name: "Test Project"},
	}

	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	outputRawJSON(projects)

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	var got []*jira.Project
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %s", err)
	}

	if len(got) != 1 || got[0].Key != "TEST" || got[0].Name != "Test Project" {
		t.Fatalf("unexpected output: %+v", got)
	}
}

func TestNewCmdListHasRawFlag(t *testing.T) {
	cmd := NewCmdList()

	flag := cmd.Flags().Lookup("raw")
	if flag == nil {
		t.Fatal("expected --raw flag to be registered")
	}
	if flag.DefValue != "false" {
		t.Fatalf("expected --raw flag default to be false, got %q", flag.DefValue)
	}
}
