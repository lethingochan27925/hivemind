// main.go: CLI don gian de dev tu test human review logic truoc khi co dashboard that.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lethingochan27925/hivemind/internal/config"
	"github.com/lethingochan27925/hivemind/internal/review"
	"github.com/lethingochan27925/hivemind/pkg/cockroach"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx := context.Background()
	db, err := cockroach.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()

	reviews, err := review.ListPendingReviews(ctx, db)
	if err != nil {
		log.Fatalf("listing pending reviews: %v", err)
	}

	if len(reviews) == 0 {
		fmt.Println("No pending reviews.")
		return
	}

	fmt.Printf("Pending reviews (%d):\n\n", len(reviews))
	for i, r := range reviews {
		fmt.Printf("[%d] task=%s verdict=%s confidence=%.2f\n", i+1, r.TaskID, r.Verdict, r.Confidence)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nEnter task number to review (or 'q' to quit): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "q" {
		return
	}

	var idx int
	fmt.Sscanf(input, "%d", &idx)
	if idx < 1 || idx > len(reviews) {
		fmt.Println("Invalid selection.")
		return
	}
	selected := reviews[idx-1]

	fmt.Printf("Task %s | verdict=%s confidence=%.2f\n", selected.TaskID, selected.Verdict, selected.Confidence)
	fmt.Print("Decision (approve/reject): ")
	decision, _ := reader.ReadString('\n')
	decision = strings.TrimSpace(decision)
	if decision != "approved" && decision != "rejected" {
		if decision == "approve" {
			decision = "approved"
		} else if decision == "reject" {
			decision = "rejected"
		} else {
			fmt.Println("Invalid decision, must be approve/reject.")
			return
		}
	}

	fmt.Print("Reviewer name: ")
	reviewer, _ := reader.ReadString('\n')
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" {
		reviewer = "cli_reviewer"
	}

	fmt.Print("Notes (optional): ")
	notes, _ := reader.ReadString('\n')
	notes = strings.TrimSpace(notes)

	if err := review.SubmitReview(ctx, db, selected.TaskID, reviewer, decision, notes); err != nil {
		log.Fatalf("submitting review: %v", err)
	}

	fmt.Printf("Review submitted: task=%s decision=%s reviewer=%s\n", selected.TaskID, decision, reviewer)
}
