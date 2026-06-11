package vipdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRequiresExplicitRoot(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("Open empty root succeeded, want error")
	}
}

func TestOpenRequiresDirectory(t *testing.T) {
	root := t.TempDir()
	file := root + "/not-a-dir"
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Open(file); err == nil {
		t.Fatal("Open file root succeeded, want error")
	}
}

func TestOpenKeepsExplicitRoot(t *testing.T) {
	root := t.TempDir()
	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if reader.Root() != root {
		t.Fatalf("Root = %q, want %q", reader.Root(), root)
	}
}

func TestBlockPathRejectsEscapingFilenames(t *testing.T) {
	root := t.TempDir()
	reader, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	cases := []string{
		"../x",
		"nested/../../x",
		filepath.Join(filepath.Dir(root), "outside.dat"),
	}
	for _, filename := range cases {
		t.Run(filename, func(t *testing.T) {
			_, err := reader.blockPath(filename)
			if err == nil {
				t.Fatal("blockPath succeeded, want invalid filename error")
			}
			for _, want := range []string{"block filename", filename} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}
