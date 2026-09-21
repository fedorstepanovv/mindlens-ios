package metrics

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Execution is what the claude-code-action's execution file said about a lane's run,
// where it said anything. Every field is optional: see LaneRecord.
type Execution struct {
	DurationMS *int64
	CostUSD    *float64
	Turns      *int
}

// resultMessage is the subset of the Claude Code SDK's final `result` message this
// reads. The action is believed to write the SDK's stream-json messages to its
// execution file, so the last message should be this one; if it is not, the reader
// yields nothing and the caller says so in the log.
type resultMessage struct {
	Type         string   `json:"type"`
	DurationMS   *int64   `json:"duration_ms"`
	TotalCostUSD *float64 `json:"total_cost_usd"`
	NumTurns     *int     `json:"num_turns"`
}

func (m resultMessage) hasNumbers() bool {
	return m.DurationMS != nil || m.TotalCostUSD != nil || m.NumTurns != nil
}

func (m resultMessage) execution() Execution {
	return Execution{DurationMS: m.DurationMS, CostUSD: m.TotalCostUSD, Turns: m.NumTurns}
}

// ReadExecution reads cost, duration and turn count off an execution file in any of
// the shapes it plausibly has: a JSON array of messages, one JSON object, or one
// message per line. An empty path is no file to read and yields nothing without
// error. A file in none of those shapes is an error, so the log says the guess was
// wrong rather than the record silently saying "not reported".
func ReadExecution(path string) (Execution, error) {
	if path == "" {
		return Execution{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Execution{}, err
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return Execution{}, fmt.Errorf("%s is empty", path)
	}

	// One array of messages.
	var array []json.RawMessage
	if json.Unmarshal(data, &array) == nil {
		return pick(array), nil
	}
	// One object, either the result message itself or something wrapping it.
	var one json.RawMessage
	if json.Unmarshal(data, &one) == nil {
		return pick([]json.RawMessage{one}), nil
	}
	// One message per line.
	var lines []json.RawMessage
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var m json.RawMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return Execution{}, fmt.Errorf("%s is not a JSON array, object, or one object per line: %w", path, err)
		}
		lines = append(lines, m)
	}
	if len(lines) == 0 {
		return Execution{}, fmt.Errorf("%s is not JSON in any shape this reader knows", path)
	}
	return pick(lines), nil
}

// pick takes the last message that carries the numbers — a `result` by type, or by
// its fields when the type is missing.
func pick(messages []json.RawMessage) Execution {
	for i := len(messages) - 1; i >= 0; i-- {
		var m resultMessage
		if json.Unmarshal(messages[i], &m) != nil {
			continue
		}
		if m.Type == "result" || (m.Type == "" && m.hasNumbers()) {
			return m.execution()
		}
	}
	return Execution{}
}
