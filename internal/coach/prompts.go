package coach

import (
	"fmt"
	"strings"
)

// SocraticSystemPrompt defines the hardened system prompt for the coach.
const SocraticSystemPrompt = `You are a world-class Socratic algorithmic interviewer and DSA coach for OpenLeet.

## CORE DIRECTIVES (NEVER VIOLATE):
1. **NEVER generate, write, or reveal full copy-pasteable code solutions.**
2. **Guide through questions and conceptual steps.** Help the user build the solution themselves.
3. **Analyze complexity ($O$ notation)**: If their code is suboptimal (e.g. O(N^2) brute force), ask questions that lead to an optimal data structure (e.g. hash map, two pointers, binary search).
4. **Identify edge cases**: Ask about empty collections, single elements, negative numbers, overflow, duplicates, or boundary limits.
5. **Be concise and direct**: Keep turns under 4 sentences unless explaining an algorithmic invariant.
6. **Tone**: Constructive, encouraging, patient, intellectually rigorous.`

// BuildUserPrompt constructs the user prompt payload from the request context.
func BuildUserPrompt(req CoachRequest) string {
	var b strings.Builder

	if req.Problem != nil {
		fmt.Fprintf(&b, "Problem #%s: %s (Difficulty: %s)\n", req.Problem.ID, req.Problem.Title, req.Problem.Difficulty)
		if len(req.Problem.Topics) > 0 {
			fmt.Fprintf(&b, "Topics: %s\n", strings.Join(req.Problem.Topics, ", "))
		}
		if req.Problem.Description != "" {
			fmt.Fprintf(&b, "\nDescription:\n%s\n", req.Problem.Description)
		}
	}

	if req.UserCode != "" {
		fmt.Fprintf(&b, "\nUser's Current %s Code:\n```%s\n%s\n```\n", req.Language, req.Language, req.UserCode)
	}

	if req.TestResults != nil && !req.TestResults.IsSuccessful() {
		b.WriteString("\nExecution Results (Failures):\n")
		if req.TestResults.StdErr != "" {
			fmt.Fprintf(&b, "Errors: %s\n", req.TestResults.StdErr)
		}
		for _, tr := range req.TestResults.Results {
			if !tr.Passed {
				fmt.Fprintf(&b, "- Test Case %s: Expected %v, got %v\n", tr.TestCaseID, tr.Expected, tr.Actual)
			}
		}
	}

	b.WriteString("\nRequest Directive:\n")
	switch req.Action {
	case ActionHint:
		b.WriteString("Provide ONE targeted hint or guiding question for the next step. Do NOT write solution code.")
	case ActionExplain:
		b.WriteString("Explain the problem requirements, key constraints, and intuition clearly in simple terms.")
	case ActionComplexity:
		b.WriteString("Audit the asymptotic Time Complexity and Space Complexity of the user's current code. Ask if they see where the bottleneck is.")
	case ActionDiagnose:
		b.WriteString("Diagnose why the code failed the test case without giving away the direct fix. Ask a question to help them trace the bug.")
	case ActionQuestion:
		if req.UserQuestion != "" {
			fmt.Fprintf(&b, "Answer this user question Socratically: %s", req.UserQuestion)
		} else {
			b.WriteString("Ask what aspect of the problem or algorithm they would like help thinking through.")
		}
	default:
		b.WriteString("Provide Socratic feedback on the current algorithm design.")
	}

	return b.String()
}
