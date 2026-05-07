// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// go command is not available on android

//go:build !android
// +build !android

package main

import (
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This file contains a test that compiles and runs each program in testdata
// after generating the string method for its type. The rule is that for testdata/x.go
// we run stringer -type X and then compile and run the program. The resulting
// binary panics if the String method for X is not correct, including for error cases.

func TestEndToEnd(t *testing.T) {
	dir := t.TempDir()

	// Create stringer in temporary directory.
	stringer := filepath.Join(dir, "stringer.exe")
	err := run("go", "build", "-o", stringer)
	if err != nil {
		t.Fatalf("building stringer: %s", err)
	}
	// Read the testdata directory.
	fd, err := os.Open("testdata")
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	names, err := fd.Readdirnames(-1)
	if err != nil {
		t.Fatalf("Readdirnames: %s", err)
	}
	// Generate, compile, and run the test programs.
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") {
			t.Errorf("%s is not a Go file", name)
			continue
		}
		if name == "cgo.go" && !build.Default.CgoEnabled {
			t.Logf("cgo is no enabled for %s", name)
			continue
		}
		// Names are known to be ASCII and long enough.
		typeName := fmt.Sprintf("%c%s", name[0]+'A'-'a', name[1:len(name)-len(".go")])
		transformNameMethod := "noop"

		if name == "transform.go" {
			typeName = "CamelCaseValue"
			transformNameMethod = "snake"
		} else if name == "transformlowercamel.go" {
			typeName = "CamelCaseValue"
			transformNameMethod = "lowercamel"
		}

		stringerCompileAndRun(t, dir, stringer, typeName, name, transformNameMethod)
	}
}

// stringerCompileAndRun runs stringer for the named file and compiles and
// runs the target binary in directory dir. That binary will panic if the String method is incorrect.
func stringerCompileAndRun(t *testing.T, dir, stringer, typeName, fileName, transformNameMethod string) {
	t.Logf("run: %s %s\n", fileName, typeName)
	source := filepath.Join(dir, fileName)
	err := copy(source, filepath.Join("testdata", fileName))
	if err != nil {
		t.Fatalf("copying file to temporary directory: %s", err)
	}
	stringSource := filepath.Join(dir, typeName+"_string.go")
	// Run stringer in temporary directory.
	err = run(stringer, "-type", typeName, "-output", stringSource, "-transform", transformNameMethod, source)
	if err != nil {
		t.Fatal(err)
	}
	// Run the binary in the temporary directory.
	err = run("go", "run", stringSource, source)
	if err != nil {
		t.Fatal(err)
	}
}

// copy copies the from file to the to file.
func copy(to, from string) error {
	toFd, err := os.Create(to)
	if err != nil {
		return err
	}
	defer toFd.Close()
	fromFd, err := os.Open(from)
	if err != nil {
		return err
	}
	defer fromFd.Close()
	_, err = io.Copy(toFd, fromFd)
	return err
}

// run runs a single command and returns an error if it does not succeed.
// os/exec should have this function, to be honest.
func run(name string, arg ...string) error {
	return runInDir(".", name, arg...)
}

// TestCommentWithNewline verifies that a -comment value containing a newline
// produces one "// " prefixed line per segment in the generated output.
func TestCommentWithNewline(t *testing.T) {
	dir := t.TempDir()

	enumerBin := filepath.Join(dir, "enumer.exe")
	if err := run("go", "build", "-o", enumerBin); err != nil {
		t.Fatalf("building enumer: %s", err)
	}

	inputSrc := `package color

type Color int

const (
	Red Color = iota
	Green
	Blue
)
`
	inputFile := filepath.Join(dir, "color.go")
	if err := os.WriteFile(inputFile, []byte(inputSrc), 0644); err != nil {
		t.Fatal(err)
	}

	outputFile := filepath.Join(dir, "color_enumer.go")
	if err := run(enumerBin, "-type", "Color", "-output", outputFile, "-comment", "line one\nline two", inputFile); err != nil {
		t.Fatalf("running enumer: %s", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("reading output: %s", err)
	}

	got := string(data)
	if !strings.Contains(got, "// line one\n") {
		t.Errorf("expected '// line one' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "// line two\n") {
		t.Errorf("expected '// line two' in output, got:\n%s", got)
	}

	// Verify the generated file is valid Go (must include the source file that defines Color).
	if err := run("go", "build", inputFile, outputFile); err != nil {
		t.Errorf("generated file does not compile: %s\ngot:\n%s", err, got)
	}
}

// runInDir runs a single command in directory dir and returns an error if
// it does not succeed.
func runInDir(dir, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
