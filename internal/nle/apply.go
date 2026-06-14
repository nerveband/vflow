package nle

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
