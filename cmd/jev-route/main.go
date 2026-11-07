package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"openleet/internal/jev"
)

func main() {
	jsonOutput := flag.Bool("json", false, "Output result as JSON")
	modelName := flag.String("model", jev.ModelFree, "Jev model to use (jev-1.13-free or jev-1.13)")
	flag.Parse()

	var taskScope string
	args := flag.Args()
	if len(args) > 0 {
		taskScope = strings.Join(args, " ")
	} else {
		// Read from stdin if piped
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err == nil {
				taskScope = strings.TrimSpace(string(data))
			}
		}
	}

	if taskScope == "" {
		fmt.Println("Usage: jev-route [flags] \"<task description>\"")
		fmt.Println("       echo \"<task description>\" | jev-route [flags]")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	client, err := jev.NewClient("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	client.SetModel(*modelName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	decision, err := client.RouteTask(ctx, taskScope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Routing failed: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(decision)
		return
	}

	// Format Neo-TUI decision banner
	fmt.Println("┌────────────────────────────────────────────────────────────────────────┐")
	fmt.Printf("│ JEV SYSTEM ONE TRIAGE GATE (TypeSafe AI • %-28s) │\n", *modelName)
	fmt.Println("├────────────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ Task Scope : %-58s │\n", truncate(decision.TaskScope, 58))
	fmt.Printf("│ Decision   : %-18s (Confidence: %3.0f%%)                  │\n",
		decision.Choice, decision.Confidence*100)
	fmt.Printf("│ Complexity : %-58s │\n", formatComplexity(decision.Complexity))
	fmt.Printf("│ Cost       : $%.4f (Free Tier Zero-Token Quota)                     │\n", decision.CostUSD)
	fmt.Println("├────────────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ Target Agent : %-56s │\n", decision.Route.TargetAgent)
	fmt.Printf("│ Model Target : %-56s │\n", decision.Route.Model)
	fmt.Printf("│ Action Scope : %-56s │\n", truncate(decision.Route.Description, 56))
	if len(decision.Route.Subagents) > 0 {
		fmt.Printf("│ Support      : %-56s │\n", strings.Join(decision.Route.Subagents, ", "))
	}
	fmt.Println("└────────────────────────────────────────────────────────────────────────┘")

	fmt.Println("\nProbabilities Distribution:")
	for choice, prob := range decision.Probabilities {
		bar := renderBar(prob, 24)
		fmt.Printf("  %-24s %s %5.1f%%\n", choice, bar, prob*100)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatComplexity(score float64) string {
	names := []string{"Trivial (0/4)", "Simple (1/4)", "Moderate (2/4)", "Complex (3/4)", "Architectural (4/4)"}
	idx := int(score + 0.5)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(names) {
		idx = len(names) - 1
	}
	return fmt.Sprintf("%.2f / 4.0  (%s)", score, names[idx])
}

func renderBar(val float64, width int) string {
	filled := int(val * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("■", filled) + strings.Repeat("·", width-filled) + "]"
}
