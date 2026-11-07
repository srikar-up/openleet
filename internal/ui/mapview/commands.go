package mapview

import (
	"fmt"
	"strconv"
	"strings"

	"openleet/internal/model"
)

// CommandResult represents the parsed outcome of a slash command.
type CommandResult struct {
	ActiveFilter  string
	TargetProblem *model.Problem
	StatusMsg     string
	OpenWorkspace bool
}

// ParseAndExecuteCommand parses and executes a slash command string.
func ParseAndExecuteCommand(input string, allProblems []*model.Problem, currentFilter string) CommandResult {
	clean := strings.TrimSpace(input)
	if strings.HasPrefix(clean, "/") {
		clean = clean[1:]
	} else if strings.HasPrefix(clean, ":") {
		clean = clean[1:]
	}
	clean = strings.TrimSpace(clean)

	if clean == "" {
		return CommandResult{ActiveFilter: currentFilter}
	}

	parts := strings.Fields(clean)
	verb := strings.ToLower(parts[0])
	args := parts[1:]

	switch verb {
	case "solve":
		if len(args) == 0 {
			return CommandResult{
				ActiveFilter: currentFilter,
				StatusMsg:    "Usage: /solve <id or title>",
			}
		}
		query := strings.ToLower(args[0])
		for _, p := range allProblems {
			if strings.ToLower(p.ID) == query ||
				strings.ToLower(p.Slug) == query ||
				strconv.Itoa(mustParseInt(p.ID)) == query {
				return CommandResult{
					ActiveFilter:  currentFilter,
					TargetProblem: p,
					StatusMsg:     fmt.Sprintf("Opened #%s: %s", p.ID, p.Title),
					OpenWorkspace: true,
				}
			}
		}
		return CommandResult{
			ActiveFilter: currentFilter,
			StatusMsg:    fmt.Sprintf("Problem '%s' not found", args[0]),
		}

	case "company":
		if len(args) == 0 {
			return CommandResult{
				ActiveFilter: currentFilter,
				StatusMsg:    "Usage: /company <name>",
			}
		}
		company := strings.ToLower(args[0])
		return CommandResult{
			ActiveFilter: "company:" + company,
			StatusMsg:    fmt.Sprintf("Filtered by company: %s", company),
		}

	case "topic":
		if len(args) == 0 {
			return CommandResult{
				ActiveFilter: currentFilter,
				StatusMsg:    "Usage: /topic <tag>",
			}
		}
		topic := strings.ToLower(args[0])
		return CommandResult{
			ActiveFilter: topic,
			StatusMsg:    fmt.Sprintf("Filtered by topic: %s", topic),
		}

	case "dsa", "all", "clear", "reset":
		return CommandResult{
			ActiveFilter: "all",
			StatusMsg:    "Filters cleared. Showing all problems.",
		}

	case "help":
		return CommandResult{
			ActiveFilter: currentFilter,
			StatusMsg:    "Commands: /solve <id> · /company <name> · /topic <name> · /dsa · /clear",
		}

	default:
		// Shorthand topic or company or ID search
		// 1. If numeric, treat as /solve <id>
		if _, err := strconv.Atoi(verb); err == nil {
			for _, p := range allProblems {
				if strings.ToLower(p.ID) == verb || strconv.Itoa(mustParseInt(p.ID)) == verb {
					return CommandResult{
						ActiveFilter:  currentFilter,
						TargetProblem: p,
						StatusMsg:     fmt.Sprintf("Opened #%s: %s", p.ID, p.Title),
						OpenWorkspace: true,
					}
				}
			}
		}

		// 2. Otherwise treat as topic filter
		return CommandResult{
			ActiveFilter: verb,
			StatusMsg:    fmt.Sprintf("Filtered by '%s'", verb),
		}
	}
}

// FilterProblems returns the subset of problems matching the active filter.
//
// Filters come in three shapes:
//   - "" / "all"          -> every problem
//   - "company:<name>"    -> problems asked by that company
//   - roadmap categories  -> problems whose Category matches (DSA, CP, DP,
//     ARRAY, LINK LIST, GRAPH)
//   - anything else       -> topic tag match
func FilterProblems(problems []*model.Problem, filter string) []*model.Problem {
	f := strings.TrimSpace(strings.ToLower(filter))
	if f == "" || f == "all" {
		return problems
	}

	if strings.HasPrefix(f, "company:") {
		targetCompany := strings.TrimPrefix(f, "company:")
		var results []*model.Problem
		for _, p := range problems {
			if containsInsensitive(p.Companies, targetCompany) {
				results = append(results, p)
			}
		}
		return results
	}

	// Difficulty filters use the same picker values as /filter.
	switch f {
	case "easy", "medium", "hard":
		var results []*model.Problem
		for _, p := range problems {
			if strings.EqualFold(string(p.Difficulty), f) {
				results = append(results, p)
			}
		}
		return results
	}

	key := model.NormalizeCategory(f)
	if isRoadmapCategory(key) {
		var results []*model.Problem
		for _, p := range problems {
			if p.CategoryKey() == key {
				results = append(results, p)
			}
		}
		return results
	}

	// Topic filter
	var results []*model.Problem
	for _, p := range problems {
		if containsInsensitive(p.Topics, f) {
			results = append(results, p)
		}
	}

	return results
}

// isRoadmapCategory reports whether the key is one of the rail categories.
func isRoadmapCategory(key string) bool {
	switch key {
	case "dsa", "cp", "dp", "array", "linklist", "graph":
		return true
	}
	return false
}

func containsInsensitive(list []string, query string) bool {
	q := strings.ToLower(query)
	for _, item := range list {
		if strings.ToLower(item) == q || strings.Contains(strings.ToLower(item), q) {
			return true
		}
	}
	return false
}

func mustParseInt(s string) int {
	val, _ := strconv.Atoi(s)
	return val
}
