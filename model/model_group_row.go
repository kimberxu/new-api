package model

import "maps"

// [personal] Row-level routing helpers.
//
// A model group member is a (channel, upstream model) row. The legacy
// channel-level selector collapsed every channel to one entry, which made
// the effective priority/weight of a channel holding several members depend
// on Go map iteration order, and made the upstream rewrite guess again
// afterwards. These helpers keep one canonical "best member" rule so the
// memory cache, the DB fallback, and the upstream resolver agree:
//
//	best = max effective priority, then max effective weight, then min model.
//
// Effective values resolve member overrides first (nil = inherit the live
// channel value), matching selectRow.priority()/weight() in the DB path.

// bestMemberOverride returns the member override that defines the channel's
// tier: max effective priority, then max effective weight, then min model.
// Nil pointer fields inherit the channel value at read time. Returns nil
// when the channel owns no member in the routable group.
func bestMemberOverride(channelId int, routable string, chanById map[int]*Channel, overrides map[string]map[string]map[int]modelGroupItemOverride) *modelGroupItemOverride {
	groupModelMap, ok := overrides[routable]
	if !ok {
		return nil
	}
	var ch *Channel
	if chanById != nil {
		ch = chanById[channelId]
	}
	var channelPri int64
	var channelW int
	if ch != nil {
		channelPri = ch.GetPriority()
		channelW = ch.GetWeight()
	}
	var best *modelGroupItemOverride
	var bestPri int64
	var bestW int
	for _, modelChanMap := range groupModelMap {
		o, ok := modelChanMap[channelId]
		if !ok {
			continue
		}
		pri := channelPri
		if o.priority != nil {
			pri = *o.priority
		}
		w := channelW
		if o.weight != nil {
			w = int(*o.weight)
		}
		if best == nil || pri > bestPri || (pri == bestPri && (w > bestW || (w == bestW && o.model < best.model))) {
			c := o
			best = &c
			bestPri = pri
			bestW = w
		}
	}
	return best
}

// filterMemberOverrides returns a copy-on-write view of the member overrides
// for one routable model with the excluded (channel, member) rows removed. The
// input is returned unchanged when no exclusion matches, so a retry that
// excludes rows of another group or model allocates nothing; untouched
// subtrees are shared, not copied.
func filterMemberOverrides(routable string, overrides map[string]map[string]map[int]modelGroupItemOverride, excluded map[int]map[string]bool) map[string]map[string]map[int]modelGroupItemOverride {
	if len(excluded) == 0 {
		return overrides
	}
	groupMembers, ok := overrides[routable]
	if !ok {
		return overrides
	}
	matched := false
	for member, chanMap := range groupMembers {
		for channelId := range chanMap {
			if excluded[channelId][member] {
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}
	if !matched {
		return overrides
	}
	copiedMembers := make(map[string]map[int]modelGroupItemOverride, len(groupMembers))
	for member, chanMap := range groupMembers {
		copied := chanMap
		for channelId := range chanMap {
			if !excluded[channelId][member] {
				continue
			}
			if len(copied) == len(chanMap) {
				copied = make(map[int]modelGroupItemOverride, len(chanMap))
				maps.Copy(copied, chanMap)
			}
			delete(copied, channelId)
		}
		copiedMembers[member] = copied
	}
	out := make(map[string]map[string]map[int]modelGroupItemOverride, len(overrides))
	maps.Copy(out, overrides)
	out[routable] = copiedMembers
	return out
}

func resolveBestUpstream(channelId int, routable string, chanById map[int]*Channel, overrides map[string]map[string]map[int]modelGroupItemOverride) string {
	best := bestMemberOverride(channelId, routable, chanById, overrides)
	if best == nil || best.model == "" || best.model == routable {
		return ""
	}
	return best.model
}
