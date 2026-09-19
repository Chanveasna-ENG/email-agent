package antigravity

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	runner := NewRunner("agy")

	// 1. Without conversation ID
	args1 := runner.BuildArgs("", "Hello world")
	expected1 := []string{"--dangerously-skip-permissions", "-p", "Hello world"}
	if !reflect.DeepEqual(args1, expected1) {
		t.Errorf("BuildArgs without conversation = %v, want %v", args1, expected1)
	}

	// 2. With conversation ID
	args2 := runner.BuildArgs("thread-abc-123", "Continue task")
	expected2 := []string{"--conversation", "thread-abc-123", "--dangerously-skip-permissions", "-p", "Continue task"}
	if !reflect.DeepEqual(args2, expected2) {
		t.Errorf("BuildArgs with conversation = %v, want %v", args2, expected2)
	}
}

func TestExecuteSuccess(t *testing.T) {
	ctx := context.Background()
	var executedName string
	var executedArgs []string

	mockExecutor := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		executedName = name
		executedArgs = args
		return []byte("\x1b[32mHere is the solution to your query.\x1b[0m\n"), nil
	}

	runner := NewRunner("agy").WithExecutor(mockExecutor)
	result, err := runner.Execute(ctx, "conv-1", "Fix the bug")
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	if executedName != "agy" {
		t.Errorf("executedName = %q, want agy", executedName)
	}
	if len(executedArgs) != 5 || executedArgs[1] != "conv-1" {
		t.Errorf("executedArgs = %v", executedArgs)
	}
	if result != "Here is the solution to your query." {
		t.Errorf("CleanOutput result = %q, want stripped ANSI text", result)
	}
}

func TestExecuteError(t *testing.T) {
	ctx := context.Background()
	mockExecutor := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("fatal: auth failed"), errors.New("exit status 1")
	}

	runner := NewRunner("agy").WithExecutor(mockExecutor)
	_, err := runner.Execute(ctx, "", "Do something")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCleanOutput(t *testing.T) {
	raw := "\x1b[31;1mError:\x1b[0m Failed to do something.   \n\n"
	cleaned := CleanOutput(raw)
	expected := "Error: Failed to do something."
	if cleaned != expected {
		t.Errorf("CleanOutput = %q, want %q", cleaned, expected)
	}
}
