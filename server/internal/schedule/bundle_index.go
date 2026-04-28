package schedule

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// BundleIndex maps content into monthly bundles, plus permanent and unreleased buckets.
type BundleIndex struct {
	Bundles    map[string]*Bundle `json:"bundles"`
	Permanent  *Bundle            `json:"permanent"`
	Unreleased *Bundle            `json:"unreleased"`
}

// Bundle represents a group of content that was released together.
type Bundle struct {
	Label         string  `json:"label"`
	EventChapters []int32 `json:"event_chapters"`
	GachaIds      []int32 `json:"gacha_ids"`
	LoginBonuses  []int32 `json:"login_bonuses"`
	SideStories   []int32 `json:"side_stories"`
	ShopIds       []int32 `json:"shop_ids"`
}

// LoadBundleIndex reads the pre-generated bundle index from a JSON file.
// This file is created by generate_bundle_index.py and maps all content
// into monthly buckets, permanent, and unreleased categories.
func LoadBundleIndex(path string) (*BundleIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bundle index %s: %w", path, err)
	}
	var idx BundleIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse bundle index %s: %w", path, err)
	}

	// Ensure no nil slices (makes downstream code simpler)
	if idx.Bundles == nil {
		idx.Bundles = make(map[string]*Bundle)
	}
	if idx.Permanent == nil {
		idx.Permanent = &Bundle{}
	}
	if idx.Unreleased == nil {
		idx.Unreleased = &Bundle{}
	}

	totalEvents, totalGacha := 0, 0
	for _, b := range idx.Bundles {
		totalEvents += len(b.EventChapters)
		totalGacha += len(b.GachaIds)
	}

	log.Printf("[schedule] bundle index loaded from %s: %d monthly bundles, %d permanent items, %d unreleased items, %d total gacha IDs",
		path, len(idx.Bundles),
		len(idx.Permanent.EventChapters),
		len(idx.Unreleased.EventChapters)+len(idx.Unreleased.GachaIds)+len(idx.Unreleased.LoginBonuses),
		totalGacha)

	return &idx, nil
}

// AllEnabledEventChapters returns the set of event chapter IDs that should be active
// given the provided schedule.
func (idx *BundleIndex) AllEnabledEventChapters(sched *ContentSchedule) int32Set {
	enabled := make(int32Set)

	// Always-enabled chapters
	for _, id := range sched.AlwaysEnabled.EventChapters {
		enabled[id] = struct{}{}
	}

	// Permanent content is always included
	for _, id := range idx.Permanent.EventChapters {
		enabled[id] = struct{}{}
	}

	// Active bundles
	activeBundles := newStringSet(sched.ActiveBundles)
	for month, bundle := range idx.Bundles {
		if activeBundles.contains(month) {
			for _, id := range bundle.EventChapters {
				enabled[id] = struct{}{}
			}
		}
	}

	// Unreleased
	if sched.UnreleasedEnabled {
		for _, id := range idx.Unreleased.EventChapters {
			enabled[id] = struct{}{}
		}
	}

	// Remove disabled overrides
	disabledChapters := newInt32Set(sched.DisabledOverrides.EventChapters)
	for id := range disabledChapters {
		delete(enabled, id)
	}

	return enabled
}

// AllEnabledGachaIds returns the set of gacha IDs that should be active.
func (idx *BundleIndex) AllEnabledGachaIds(sched *ContentSchedule) int32Set {
	enabled := make(int32Set)

	activeBundles := newStringSet(sched.ActiveBundles)
	for month, bundle := range idx.Bundles {
		if activeBundles.contains(month) {
			for _, id := range bundle.GachaIds {
				enabled[id] = struct{}{}
			}
		}
	}

	if sched.UnreleasedEnabled {
		for _, id := range idx.Unreleased.GachaIds {
			enabled[id] = struct{}{}
		}
	}

	disabledGacha := newInt32Set(sched.DisabledOverrides.GachaIds)
	for id := range disabledGacha {
		delete(enabled, id)
	}

	return enabled
}

// AllEnabledLoginBonuses returns the set of login bonus IDs that should be active.
func (idx *BundleIndex) AllEnabledLoginBonuses(sched *ContentSchedule) int32Set {
	enabled := make(int32Set)

	activeBundles := newStringSet(sched.ActiveBundles)
	for month, bundle := range idx.Bundles {
		if activeBundles.contains(month) {
			for _, id := range bundle.LoginBonuses {
				enabled[id] = struct{}{}
			}
		}
	}

	if sched.UnreleasedEnabled {
		for _, id := range idx.Unreleased.LoginBonuses {
			enabled[id] = struct{}{}
		}
	}

	disabledLB := newInt32Set(sched.DisabledOverrides.LoginBonusIds)
	for id := range disabledLB {
		delete(enabled, id)
	}

	return enabled
}
