/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
)

// TestDecideModelBanRecovery covers the pure decision logic used by the
// expired model-ban recovery pass: only expired auto bans are eligible, a
// successful probe clears the record, a failed probe renews the deadline.
func TestDecideModelBanRecovery(t *testing.T) {
	tests := []struct {
		name     string
		record   *model.ChannelDisabledModel
		probeOK  bool
		wantRecovered bool
		wantExtended  bool
		wantSkipped   bool
	}{
		{
			name:            "expired auto ban probe ok recovers",
			record:          &model.ChannelDisabledModel{Source: "auto", BannedUntil: 100},
			probeOK:         true,
			wantRecovered:   true,
		},
		{
			name:            "expired auto ban probe fail extends",
			record:          &model.ChannelDisabledModel{Source: "auto", BannedUntil: 100},
			probeOK:         false,
			wantExtended:    true,
		},
		{
			name:            "manual ban is skipped even when expired",
			record:          &model.ChannelDisabledModel{Source: "manual", BannedUntil: 100},
			probeOK:         true,
			wantSkipped:     true,
		},
		{
			name:            "permanent auto ban is skipped",
			record:          &model.ChannelDisabledModel{Source: "auto", BannedUntil: 0},
			probeOK:         true,
			wantSkipped:     true,
		},
		{
			name:            "nil record is skipped",
			record:          nil,
			probeOK:         true,
			wantSkipped:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := decideModelBanRecovery(tt.record, tt.probeOK)
			assert.Equal(t, tt.wantRecovered, decision.Recovered)
			assert.Equal(t, tt.wantExtended, decision.Extended)
			assert.Equal(t, tt.wantSkipped, decision.Skipped)
		})
	}
}