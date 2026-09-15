package picker

import (
	"strings"
	"testing"

	"github.com/tryoverflow/ssmhop/internal/target"
)

func TestHeaderShowsAccountContext(t *testing.T) {
	got := Options{Region: "eu-west-1", Profile: "prod"}.header()
	for _, want := range []string{"region (eu-west-1)", "profile (prod)", "type to filter"} {
		if !strings.Contains(got, want) {
			t.Errorf("header() = %q, want it to contain %q", got, want)
		}
	}

	if got := (Options{Region: "eu-west-1"}).header(); strings.Contains(got, "profile") {
		t.Errorf("header() = %q, want no profile section", got)
	}
}

func TestPromptArticle(t *testing.T) {
	tests := map[string]string{
		"target":    "Choose a target in AWS: ",
		"instance":  "Choose an instance in AWS: ",
		"container": "Choose a container in AWS: ",
		"":          "Choose a target in AWS: ",
	}
	for noun, want := range tests {
		if got := (Options{Noun: noun}).prompt(); got != want {
			t.Errorf("prompt() for %q = %q, want %q", noun, got, want)
		}
	}
}

func TestPickRejectsEmptyList(t *testing.T) {
	if _, err := Pick(nil, Options{Noun: "target"}); err == nil {
		t.Fatal("Pick(nil) = nil error, want an error")
	}
	if _, err := Pick([]target.Target{}, Options{Noun: "target"}); err == nil {
		t.Fatal("Pick(empty) = nil error, want an error")
	}
}
