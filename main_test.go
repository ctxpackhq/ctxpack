package main

import (
	"math"
	"testing"
)

func TestScoreFilesPenalizesGoTestFiles(t *testing.T) {
	files := []fileEntry{
		{
			path:    "internal/auth/login.go",
			content: "package auth\n\nfunc Login() { authenticate user token login }\n",
		},
		{
			path:    "internal/auth/login_test.go",
			content: "package auth\n\nfunc Login() { authenticate user token login }\n",
		},
	}

	scored := scoreFiles(files, "login authenticate token")

	var sourceScore, testScore float64
	for _, f := range scored {
		switch f.path {
		case "internal/auth/login.go":
			sourceScore = f.score
		case "internal/auth/login_test.go":
			testScore = f.score
		}
	}

	if sourceScore == 0 || testScore == 0 {
		t.Fatalf("expected non-zero scores, got source=%f test=%f", sourceScore, testScore)
	}
	if math.Abs(testScore-sourceScore*0.3) > 1e-12 {
		t.Fatalf("expected test score to be 0.3x source score, got source=%f test=%f", sourceScore, testScore)
	}
}
