package internal

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// OutputStats outputs processing statistics
func OutputStats(mediaType string, stats ProcessingStats) {
	summaryFile := os.Getenv("GITHUB_STEP_SUMMARY")

	title := strings.ToUpper(mediaType[:1]) + mediaType[1:]
	diff := stats.TotalAfter - stats.TotalBefore
	diffStr := fmt.Sprintf("%+d", diff)
	if diff >= 0 {
		diffStr = "+" + fmt.Sprintf("%d", diff)
	}

	output := fmt.Sprintf("\n## %s - Summary\n\n", title)
	output += "| Metric | Before | After | Diff |\n|--------|--------|-------|------|\n"
	output += fmt.Sprintf("| Total Entries | %d | %d | %s |\n", stats.TotalBefore, stats.TotalAfter, diffStr)
	output += fmt.Sprintf("| Created | - | %d | +%d |\n", stats.Created, stats.Created)
	output += fmt.Sprintf("| Updated | - | %d | +%d |\n", stats.Updated, stats.Updated)
	output += fmt.Sprintf("| Modified (Overridden) | - | %d | +%d |\n", stats.Modified, stats.Modified)
	output += fmt.Sprintf("| Not Found | - | %d | +%d |\n", stats.NotFound, stats.NotFound)

	if len(stats.CreatedDetails) > 0 {
		output += fmt.Sprintf("\n### ✨ Created (%d)\n\n", len(stats.CreatedDetails))
		output += "| Title | MAL ID | Reason |\n|-------|--------|--------|\n"
		for _, detail := range stats.CreatedDetails {
			output += fmt.Sprintf("| %s | %d | %s |\n", detail.Title, detail.MalID, detail.Reason)
		}
	}

	if len(stats.UpdatedDetails) > 0 {
		output += fmt.Sprintf("\n### 🔄 Updated (%d)\n\n", len(stats.UpdatedDetails))
		output += "| Title | MAL ID | Reason |\n|-------|--------|--------|\n"
		for _, detail := range stats.UpdatedDetails {
			output += fmt.Sprintf("| %s | %d | %s |\n", detail.Title, detail.MalID, detail.Reason)
		}
	}

	if len(stats.ModifiedDetails) > 0 {
		output += fmt.Sprintf("\n### 🔧 Modified via Override (%d)\n\n", len(stats.ModifiedDetails))
		output += "| Title | MAL ID | Reason |\n|-------|--------|--------|\n"
		for _, detail := range stats.ModifiedDetails {
			output += fmt.Sprintf("| %s | %d | %s |\n", detail.Title, detail.MalID, detail.Reason)
		}
	}

	if len(stats.NotFoundDetails) > 0 {
		output += fmt.Sprintf("\n### ❌ Not Found (%d)\n\n", len(stats.NotFoundDetails))
		output += "| Title | MAL ID | Reason |\n|-------|--------|--------|\n"
		for _, detail := range stats.NotFoundDetails {
			output += fmt.Sprintf("| %s | %d | %s |\n", detail.Title, detail.MalID, detail.Reason)
		}
	}

	if summaryFile != "" {
		f, err := os.OpenFile(summaryFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Warning: Could not write to GITHUB_STEP_SUMMARY: %v", err)
			return
		}
		defer f.Close()
		f.WriteString(output)
	} else {
		fmt.Println(output)
	}
}
