package model

import (
	"errors"
	"math/rand/v2"

	"github.com/QuantumNous/new-api/common"
	channelslowstream "github.com/QuantumNous/new-api/pkg/channel_slowstream"
)

// selectRow is one candidate member: a (channel, upstream model) pair from an
// enabled model group, with member-level overrides and channel fallbacks.
type selectRow struct {
	ChannelId       int
	Model           string
	ItemPriority    *int64
	ItemWeight      *uint
	ChannelPriority *int64
	ChannelWeight   *uint
}

func (r selectRow) priority() int64 {
	if r.ItemPriority != nil {
		return *r.ItemPriority
	}
	if r.ChannelPriority != nil {
		return *r.ChannelPriority
	}
	return 0
}

func (r selectRow) weight() int {
	if r.ItemWeight != nil {
		return int(*r.ItemWeight)
	}
	if r.ChannelWeight != nil {
		return int(*r.ChannelWeight)
	}
	return 0
}

// [personal] GetRandomSatisfiedChannelFromGroups is the DB fallback selector
// (MemoryCache disabled) driven by model groups instead of the abilities
// table. Members with the highest effective priority —
// COALESCE(member.priority, channel.priority) — are picked, and within that
// tier a member is drawn weighted by COALESCE(member.weight, channel.weight).
// Excluded channels (already tried in the retry loop) are dropped before the
// tier computation. `group` is the channel group field, `model` the routable
// model name (= model group name). Returns the selected channel and its
// best-member upstream model ("" when no rewrite is needed).
func GetRandomSatisfiedChannelFromGroups(group string, model string, excludeChannels []int) (*Channel, string, error) {
	q := DB.Table("model_group_items").
		Select("model_group_items.channel_id as channel_id, model_group_items.model as model, "+
			"model_group_items.priority as item_priority, model_group_items.weight as item_weight, "+
			"channels.priority as channel_priority, channels.weight as channel_weight").
		Joins("JOIN model_groups ON model_group_items.group_id = model_groups.id").
		Joins("JOIN channels ON model_group_items.channel_id = channels.id").
		Where("model_groups.name = ? AND model_groups.enabled = ? AND model_group_items.enabled = ? AND channels.status = ? AND channels."+commonGroupCol+" = ?",
			model, true, true, common.ChannelStatusEnabled, group)
	if len(excludeChannels) > 0 {
		q = q.Where("model_group_items.channel_id NOT IN ?", excludeChannels)
	}
	// Exclude (channel, model) pairs that are model-level disabled. The
	// subquery is dialect-neutral (SQLite/MySQL/PostgreSQL).
	q = q.Where("NOT EXISTS (SELECT 1 FROM channel_disabled_models WHERE channel_id = model_group_items.channel_id AND model = model_group_items.model)")

	var rows []selectRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, "", err
	}
	if len(rows) == 0 {
		return nil, "", nil
	}

	// Aggregate to one best member per channel: max effective priority,
	// then max effective weight, then min model — same rule as cache path
	// bestMemberOverride. This prevents a channel with N members from
	// counting its weight N times and keeps DB/cache tier semantics aligned.
	bestByChannel := make(map[int]selectRow, len(rows))
	for _, r := range rows {
		if cur, ok := bestByChannel[r.ChannelId]; !ok {
			bestByChannel[r.ChannelId] = r
		} else {
			pr, pw := r.priority(), r.weight()
			cpr, cpw := cur.priority(), cur.weight()
			if pr > cpr || (pr == cpr && (pw > cpw || (pw == cpw && r.Model < cur.Model))) {
				bestByChannel[r.ChannelId] = r
			}
		}
	}
	bestRows := make([]selectRow, 0, len(bestByChannel))
	for _, r := range bestByChannel {
		bestRows = append(bestRows, r)
	}

	// Highest effective priority tier, with slow-stream demotion applied
	// once per channel on the aggregated best priority (mirrors cache path).
	effectivePriority := func(r selectRow) int64 {
		p := r.priority()
		if demoted, dp := channelslowstream.GetDemotedPriority(r.ChannelId, model, p); demoted {
			return dp
		}
		return p
	}
	highest := effectivePriority(bestRows[0])
	for _, r := range bestRows[1:] {
		if p := effectivePriority(r); p > highest {
			highest = p
		}
	}

	type draw struct {
		channelId int
		weight    int
		model     string
	}
	var top []draw
	sumWeight := 0
	for _, r := range bestRows {
		if effectivePriority(r) == highest {
			w := r.weight()
			sumWeight += w
			top = append(top, draw{r.ChannelId, w, r.Model})
		}
	}
	if len(top) == 0 {
		return nil, "", errors.New("no channel found, group: " + group + ", model: " + model)
	}

	// Weighted random draw, mirroring the memory-path smoothing: when the
	// weight sum is 0 every member gets an equal effective weight of 100.
	if sumWeight == 0 {
		sumWeight = len(top) * 100
		for i := range top {
			top[i].weight = 100
		}
	}
	drawN := rand.IntN(sumWeight)
	pickedIdx := -1
	for i, t := range top {
		drawN -= t.weight
		if drawN < 0 {
			pickedIdx = i
			break
		}
	}
	if pickedIdx < 0 {
		return nil, "", errors.New("no channel found, group: " + group + ", model: " + model)
	}
	picked := top[pickedIdx]
	// selectAll=true: the relay needs the key for the upstream Authorization
	// header; the legacy DB path also loaded the full row.
	ch, err := GetChannelById(picked.channelId, true)
	if err != nil {
		return nil, "", err
	}
	upstream := picked.model
	if upstream == model {
		upstream = ""
	}
	return ch, upstream, nil
}