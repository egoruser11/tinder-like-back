package repository

import (
	"strings"
	"testing"

	"tinder-core/internal/geo"
	"tinder-core/internal/models"
)

func TestBuildCandidateQuery_AddsOnlyConfiguredOptionalFilters(t *testing.T) {
	city := "Москва"
	gender := int16(2)
	query, args := buildCandidateQuery(CandidateQuery{
		ViewerUserID: 7,
		Filters: models.DiscoveryPreferences{
			MinAge:     18,
			MaxAge:     35,
			City:       &city,
			Gender:     &gender,
			IsVerified: true,
		},
		Bounds: geo.Bounds{
			MinLatitude:  54,
			MaxLatitude:  56,
			MinLongitude: 36,
			MaxLongitude: 39,
		},
		AfterProfileID: 10,
		Limit:          30,
	})

	for _, condition := range []string{
		"p.city = $9",
		"p.gender = $10",
		"u.is_verified = TRUE",
		"e.action_user_id = $1 AND e.target_user_id = p.user_id",
		"e.action_user_id = p.user_id AND e.target_user_id = $1",
		"LIMIT $11",
	} {
		if !strings.Contains(query, condition) {
			t.Fatalf("query does not contain %q:\n%s", condition, query)
		}
	}
	if len(args) != 11 {
		t.Fatalf("args length = %d, want 11", len(args))
	}
	if args[8] != city || args[9] != gender || args[10] != 30 {
		t.Fatalf("unexpected optional arguments: %#v", args[8:])
	}
}

func TestBuildCandidateQuery_SkipsNilOptionalFilters(t *testing.T) {
	query, args := buildCandidateQuery(CandidateQuery{
		ViewerUserID: 1,
		Filters: models.DiscoveryPreferences{
			MinAge: 18,
			MaxAge: 99,
		},
		Bounds: geo.Bounds{},
		Limit:  20,
	})

	if strings.Contains(query, "p.city =") || strings.Contains(query, "p.gender =") {
		t.Fatalf("query unexpectedly contains optional filters:\n%s", query)
	}
	if len(args) != 9 || args[8] != 20 {
		t.Fatalf("unexpected arguments: %#v", args)
	}
}
