package common

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
)

func TestGetOutboundModelName(t *testing.T) {
	assert.Equal(t, "", (*RelayInfo)(nil).GetOutboundModelName())

	originOnly := &RelayInfo{OriginModelName: "all-text-only"}
	assert.Equal(t, "all-text-only", originOnly.GetOutboundModelName())

	mapped := &RelayInfo{
		OriginModelName: "all-text-only",
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "muse-spark-1.3-contributor-free",
			IsModelMapped:     true,
		},
	}
	assert.Equal(t, "muse-spark-1.3-contributor-free", mapped.GetOutboundModelName())
}

func TestGetOutboundModelNameIgnoresRequestModelField(t *testing.T) {
	// The request DTO model may lag behind mapping (handlers DeepCopy before
	// ModelMappedHelper); the outbound name must come from ChannelMeta.
	info := &RelayInfo{
		OriginModelName: "all-text-only",
		Request:         &dto.GeneralOpenAIRequest{Model: "all-text-only"},
		ChannelMeta: &ChannelMeta{
			UpstreamModelName: "muse-spark-1.3-contributor-free",
			IsModelMapped:     true,
		},
	}
	assert.Equal(t, "muse-spark-1.3-contributor-free", info.GetOutboundModelName())
}
