package controller

import (
	"math/rand/v2"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/relaykit/dto"
)

const (
	// capturedMessageMaxCount 每个渠道环形缓冲上限
	capturedMessageMaxCount = 4
	// capturedMessageMaxLen 单条文本截断长度（字符）
	capturedMessageMaxLen = 200
)

// globalCapturedPool 真实请求测活消息池，key = channelID。
// 每个渠道存储最近 capturedMessageMaxCount 条提取消息序列。
var globalCapturedPool = struct {
	mu   sync.RWMutex
	pool map[int][][]dto.Message
}{pool: make(map[int][][]dto.Message)}

// extractTestMessages 从 chat 请求提取可用于重建测活请求的消息序列。
//
// 规则：
//   - 跳过 role == "tool" 的消息、含 ToolCalls 的 assistant 消息
//   - 文本取自 content 字符串或 type == "text" 的 parts（图片/音频 parts 丢弃）
//   - 单条文本截断至 capturedMessageMaxLen 字符
//   - 以最后一条含文本的 user 消息为终点，向前取最近至多 4 条有效消息
//   - 不足 1 条有效消息 → 返回 nil
func extractTestMessages(req *dto.GeneralOpenAIRequest) []dto.Message {
	if req == nil || len(req.Messages) == 0 {
		return nil
	}

	// 第一遍：过滤有效消息并提取文本
	type entry struct {
		msg  dto.Message
		text string
	}
	var valid []entry
	for _, m := range req.Messages {
		if m.Role == "tool" {
			continue
		}
		if len(m.ToolCalls) > 0 {
			continue
		}
		text := extractTextContent(&m)
		if text == "" {
			continue
		}
		// 截断至上限
		runes := []rune(text)
		if len(runes) > capturedMessageMaxLen {
			runes = runes[:capturedMessageMaxLen]
		}
		valid = append(valid, entry{msg: m, text: string(runes)})
	}

	if len(valid) == 0 {
		return nil
	}

	// 第二遍：从后向前，以最后一条 user 消息为终点，取最近至多 4 条
	lastUserIdx := -1
	for i := len(valid) - 1; i >= 0; i-- {
		if valid[i].msg.Role == "user" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		return nil
	}

	start := lastUserIdx - (capturedMessageMaxCount - 1)
	if start < 0 {
		start = 0
	}
	selected := valid[start : lastUserIdx+1]

	// 重建消息列表，保持原始 role 和文本
	result := make([]dto.Message, len(selected))
	for i, e := range selected {
		msg := dto.Message{
			Role:    e.msg.Role,
			Content: e.text,
		}
		// 保留 Name 以增强真实性
		if e.msg.Name != nil {
			msg.Name = e.msg.Name
		}
		result[i] = msg
	}
	return result
}

// extractTextContent 从消息中提取文本内容。
func extractTextContent(m *dto.Message) string {
	if m.IsStringContent() {
		return m.StringContent()
	}
	parts := m.ParseContent()
	var sb strings.Builder
	for _, p := range parts {
		if p.Type == dto.ContentTypeText {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// CaptureTestMessages 捕获请求消息：断言 *dto.GeneralOpenAIRequest 成功且
// extractTestMessages 非 nil 时写入环形缓冲（超上限移除最旧）。
func CaptureTestMessages(channelID int, request dto.Request) {
	req, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return
	}
	messages := extractTestMessages(req)
	if messages == nil {
		return
	}

	globalCapturedPool.mu.Lock()
	defer globalCapturedPool.mu.Unlock()

	pool := globalCapturedPool.pool[channelID]
	pool = append(pool, messages)
	if len(pool) > capturedMessageMaxCount {
		pool = pool[len(pool)-capturedMessageMaxCount:]
	}
	globalCapturedPool.pool[channelID] = pool
}

// GetCapturedTestMessages 随机返回该渠道一条捕获消息序列；无记录返回 nil。
func GetCapturedTestMessages(channelID int) []dto.Message {
	globalCapturedPool.mu.RLock()
	defer globalCapturedPool.mu.RUnlock()

	pool := globalCapturedPool.pool[channelID]
	if len(pool) == 0 {
		return nil
	}
	idx := rand.IntN(len(pool))
	tmp := append([]dto.Message(nil), pool[idx]...)
	return tmp
}

// ClearCapturedTestMessages 清除该渠道全部捕获。
func ClearCapturedTestMessages(channelID int) {
	globalCapturedPool.mu.Lock()
	defer globalCapturedPool.mu.Unlock()
	delete(globalCapturedPool.pool, channelID)
}