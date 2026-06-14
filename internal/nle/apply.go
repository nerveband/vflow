package nle

import (
	"fmt"
	"strings"
)

func PlanApply(diff DiffResult, acceptNeedsReview bool) ApplyPlan {
	plan := ApplyPlan{
		Version:     "vflow-nle-apply/v1",
		Status:      "planned",
		Applied:     []Change{},
		NeedsReview: diff.NeedsReview,
		Blocked:     diff.Blocked,
	}
	if len(diff.Blocked) > 0 || len(diff.Unclassified) > 0 {
		plan.Status = "blocked"
		return plan
	}
	plan.Applied = append(plan.Applied, diff.SafeMerge...)
	if len(diff.NeedsReview) > 0 {
		if !acceptNeedsReview {
			plan.Status = "needs_review"
			return plan
		}
		plan.Applied = append(plan.Applied, diff.NeedsReview...)
		plan.NeedsReview = []Change{}
	}
	return plan
}

func BuildAcceptedReview(diff DiffResult, changeIDs []string, acceptAllNeedsReview bool, reviewer, notes string) (AcceptedReview, error) {
	acceptedIDs := map[string]bool{}
	for _, id := range changeIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			acceptedIDs[id] = true
		}
	}
	if !acceptAllNeedsReview && len(acceptedIDs) == 0 && len(diff.NeedsReview) > 0 {
		return AcceptedReview{}, fmt.Errorf("select at least one --change-id or pass --all-needs-review")
	}
	review := AcceptedReview{
		Version:             "vflow-nle-accepted-review/v1",
		Status:              "accepted",
		Source:              diff.Import,
		Format:              diff.Format,
		Reviewer:            strings.TrimSpace(reviewer),
		Notes:               strings.TrimSpace(notes),
		SafeMerge:           append([]Change{}, diff.SafeMerge...),
		AcceptedNeedsReview: []Change{},
		RejectedNeedsReview: []Change{},
		Blocked:             append([]Change{}, diff.Blocked...),
		Unclassified:        append([]Change{}, diff.Unclassified...),
	}
	for _, change := range diff.NeedsReview {
		if acceptAllNeedsReview || acceptedIDs[change.ID] {
			review.AcceptedNeedsReview = append(review.AcceptedNeedsReview, change)
		} else {
			review.RejectedNeedsReview = append(review.RejectedNeedsReview, change)
		}
	}
	return review, nil
}

func PlanApplyAccepted(review AcceptedReview) ApplyPlan {
	plan := ApplyPlan{
		Version:     "vflow-nle-apply/v1",
		Status:      "planned",
		Applied:     []Change{},
		NeedsReview: []Change{},
		Blocked:     append([]Change{}, review.Blocked...),
	}
	if len(review.Blocked) > 0 || len(review.Unclassified) > 0 {
		plan.Status = "blocked"
		return plan
	}
	plan.Applied = append(plan.Applied, review.SafeMerge...)
	plan.Applied = append(plan.Applied, review.AcceptedNeedsReview...)
	return plan
}
