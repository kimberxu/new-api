package model

import "errors"

// SyncModelGroupForChannel aggregates a channel's models into model groups
// (group name = routable model name). It is idempotent: existing members keep
// their manual priority/weight/enabled edits (OnConflict DoNothing), and
// members whose model was removed from the channel are deleted.
func SyncModelGroupForChannel(channel *Channel) error {
	models := channel.GetModels()
	if len(models) == 0 {
		return DeleteModelGroupItemsNotIn(channel.Id, nil)
	}
	groups, err := GetModelGroupByNameMap(models)
	if err != nil {
		return err
	}
	// items grouped by group id: AddModelGroupItems overwrites the item's
	// GroupId with its groupId argument, so each group needs its own call.
	itemsByGroup := make(map[int][]ModelGroupItem)
	for _, m := range models {
		g, ok := groups[m]
		if !ok {
			g, err = CreateModelGroup(m, GroupSourceAuto)
			if err != nil {
				// duplicate in a concurrent sync: reuse it
				g, err = GetModelGroupByName(m)
				if err != nil {
					return err
				}
			}
		}
		if g == nil {
			return errors.New("model group not found after create/reuse")
		}
		itemsByGroup[g.Id] = append(itemsByGroup[g.Id], ModelGroupItem{
			ChannelId: channel.Id,
			Model:     m,
			Priority:  nil, // inherit channel priority
			Weight:    nil, // inherit channel weight
			Enabled:   true,
		})
	}
	for groupId, items := range itemsByGroup {
		if err := AddModelGroupItems(groupId, items); err != nil {
			return err
		}
	}
	// Remove members whose model no longer exists on the channel. Only after
	// the inserts so concurrent updates never lose the just-added rows.
	return DeleteModelGroupItemsNotIn(channel.Id, models)
}

// SyncAllModelGroups reconciles every channel's models into model groups.
// Idempotent; safe to run on startup and periodic full cache refresh.
func SyncAllModelGroups() error {
	channels, err := GetAllChannels(0, 0, true, false)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if err := SyncModelGroupForChannel(channel); err != nil {
			return err
		}
	}
	return nil
}