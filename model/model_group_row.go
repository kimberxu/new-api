package model

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

func resolveBestUpstream(channelId int, routable string, chanById map[int]*Channel, overrides map[string]map[string]map[int]modelGroupItemOverride) string {
	best := bestMemberOverride(channelId, routable, chanById, overrides)
	if best == nil || best.model == "" || best.model == routable {
		return ""
	}
	return best.model
}
