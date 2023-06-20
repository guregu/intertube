package tube

import (
	"fmt"
	"sort"
)

type PlanKind string

const (
	PlanKindNone  PlanKind = ""
	PlanKindSmall PlanKind = "small"
	PlanKindBig   PlanKind = "big"
	PlanKindHuge  PlanKind = "huge"
)

func (pk PlanKind) Msg() string {
	return "plan_" + string(pk)
}

type Plan struct {
	Kind    PlanKind
	Quota   int64
	PriceID string
}

var plans = map[PlanKind]Plan{
	PlanKindNone: {
		Kind:  PlanKindNone,
		Quota: 50 * 1024 * 1024 * 1024, // 50GB
	},
	PlanKindSmall: {
		Kind:  PlanKindSmall,
		Quota: 250 * 1024 * 1024 * 1024, // 250GB
		//PriceID: "price_1I1FzcKpetgr0YLEljmXzSVH",
		PriceID: "price_1I9kyKKpetgr0YLEzm17RXIp",
	},
	PlanKindBig: {
		Kind:  PlanKindBig,
		Quota: 500 * 1024 * 1024 * 1024, // 500GB
		//PriceID: "price_1I1G0TKpetgr0YLEyd1kx5DQ",
		PriceID: "price_1I9kyDKpetgr0YLERiDJn7Kf",
	},
	PlanKindHuge: {
		Kind:  PlanKindHuge,
		Quota: 2 * 1024 * 1024 * 1024 * 1024, // 2TB
		// PriceID: "price_1I1G5gKpetgr0YLEj0xgvqiw",
		PriceID: "price_1I9ky7Kpetgr0YLEXAiN8Kfy",
	},
}

func GetPlan(kind PlanKind) Plan {
	plan, ok := plans[kind]
	if !ok {
		panic(fmt.Errorf("no such plan: %v", kind))
	}
	return plan
}

func GetPlans() []Plan {
	var all []Plan
	for _, p := range plans {
		if p.Kind == PlanKindNone {
			continue
		}
		all = append(all, p)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Quota < all[j].Quota
	})
	return all
}

type PlanStatus string

const (
	PlanStatusActive            PlanStatus = "active"
	PlanStatusTrialing          PlanStatus = "trialing"
	PlanStatusIncomplete        PlanStatus = "incomplete"
	PlanStatusIncompleteExpired PlanStatus = "incomplete_expired"
	PlanStatusPastDue           PlanStatus = "past_due"
	PlanStatusCanceled          PlanStatus = "canceled"
	PlanStatusUnpaid            PlanStatus = "unpaid"
)

func (ps PlanStatus) Active() bool {
	switch ps {
	case PlanStatusActive, PlanStatusTrialing:
		return true
	case PlanStatusCanceled:
		// TODO: double check
		return false
	}
	return false
}
