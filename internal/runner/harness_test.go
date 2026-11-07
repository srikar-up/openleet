package runner

import (
	"strings"
	"testing"

	"openleet/internal/model"
)

func TestHarnessPython(t *testing.T) {
	h := NewHarness()
	prob := &model.Problem{
		ID:   "001",
		Slug: "two_sum",
		TestCases: []model.TestCase{
			{ID: "tc1", Input: map[string]interface{}{"nums": []int{2, 7, 11, 15}, "target": 9}, Expected: []int{0, 1}},
		},
	}

	userCode := `
class Solution:
    def twoSum(self, nums, target):
        return [0, 1]
`
	src, err := h.Generate("python", prob, userCode)
	if err != nil {
		t.Fatalf("failed to generate python harness: %v", err)
	}

	if !strings.Contains(src, "class Solution") {
		t.Fatal("expected generated code to contain user solution")
	}
	if !strings.Contains(src, "json.dump") {
		t.Fatal("expected generated code to write json to stdout")
	}
}

func TestHarnessUnsupported(t *testing.T) {
	h := NewHarness()
	prob := &model.Problem{ID: "001"}
	_, err := h.Generate("fortran", prob, "code")
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}
