package model

// [personal] Row-level retry exclusions.
//
// A failure belongs to the member row (channel, upstream model), not to the
// channel: excluding the whole channel after one member fails would discard its
// healthy siblings and cascade straight to a lower priority tier. The retry
// loop therefore accumulates ExcludedChannelModel entries and the selectors
// drop exactly those rows, keeping sibling members eligible.

// ExcludedChannelModel identifies one routing candidate to skip while
// re-selecting a channel in the current request's retry loop. Model == ""
// excludes the whole channel (rate-limit path); a non-empty Model excludes
// only that member row of the channel, keeping its sibling members eligible.
type ExcludedChannelModel struct {
	ChannelId int
	Model     string
}

// buildExcludeSets splits a retry exclusion list into whole-channel ids and
// per-channel member sets for O(1) lookups during selection.
func buildExcludeSets(exclude []ExcludedChannelModel) (whole map[int]bool, members map[int]map[string]bool) {
	for _, e := range exclude {
		if e.Model == "" {
			if whole == nil {
				whole = make(map[int]bool)
			}
			whole[e.ChannelId] = true
			continue
		}
		if members == nil {
			members = make(map[int]map[string]bool)
		}
		if members[e.ChannelId] == nil {
			members[e.ChannelId] = make(map[string]bool)
		}
		members[e.ChannelId][e.Model] = true
	}
	return whole, members
}

// memberModel returns the best remaining member's upstream model for the
// chosen channel under the (filtered) overrides, verbatim. Returns "" when the
// channel owns no member row for the routable model; callers then fall back to
// whole-channel exclusion.
func memberModel(channelId int, routable string, chanById map[int]*Channel, overrides map[string]map[string]map[int]modelGroupItemOverride) string {
	if best := bestMemberOverride(channelId, routable, chanById, overrides); best != nil {
		return best.model
	}
	return ""
}
