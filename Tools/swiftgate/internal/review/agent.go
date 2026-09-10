package review

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/gate"
	"github.com/fedorstepanovv/mindlens-ios/Tools/swiftgate/internal/scan"
)

// Meta is what the gate knows about the pull request itself.
type Meta struct {
	Title string
	Body  string
}

// Config drives one review.
type Config struct {
	RepoDir    string
	FlutterDir string // "" when the spec repo was not checked out
	Model      string
	Effort     string
	MaxTurns   int
	Log        io.Writer
}

// Review is the agentic pass. It returns the model's findings and what the run cost.
func Review(ctx context.Context, cfg Config, d scan.Diff, meta Meta, alreadyFound []gate.Finding) ([]gate.Finding, string, gate.Usage, error) {
	ws, err := newWorkspace(cfg.RepoDir, cfg.FlutterDir)
	if err != nil {
		return nil, "", gate.Usage{}, err
	}

	sink := &collector{}
	tools, err := buildTools(ws, sink)
	if err != nil {
		return nil, "", gate.Usage{}, err
	}

	client := anthropic.NewClient(option.WithMaxRetries(3))

	params := anthropic.BetaToolRunnerParams{
		BetaMessageNewParams: anthropic.BetaMessageNewParams{
			Model:     anthropic.Model(cfg.Model),
			MaxTokens: 32000,
			System:    System(cfg.RepoDir),
			Thinking: anthropic.BetaThinkingConfigParamUnion{
				OfAdaptive: &anthropic.BetaThinkingConfigAdaptiveParam{},
			},
			OutputConfig: anthropic.BetaOutputConfigParam{
				Effort: anthropic.BetaOutputConfigEffort(cfg.Effort),
			},
			Messages: []anthropic.BetaMessageParam{
				anthropic.NewBetaUserMessage(
					anthropic.NewBetaTextBlock(Task(d, meta, alreadyFound, ws.flutter != "")),
				),
			},
		},
		MaxIterations: cfg.MaxTurns,
	}

	runner := client.Beta.Messages.NewToolRunner(tools, params)

	var usage gate.Usage
	usage.Model = cfg.Model

	// Step the runner rather than RunToCompletion so the run can be cut short the
	// moment the verdict lands, and so token spend is accumulated across every turn
	// instead of read off the last message only.
	for message, err := range runner.All(ctx) {
		if err != nil {
			// A partial review that found real blockers is still worth reporting;
			// only a review that found nothing is a hard failure.
			if len(sink.findings) > 0 {
				fmt.Fprintf(cfg.Log, "review ended early (%v) — reporting %d finding(s) gathered so far\n", err, len(sink.findings))
				break
			}
			return nil, "", usage, fmt.Errorf("review failed: %w", err)
		}
		if message == nil {
			break
		}

		usage.Turns++
		usage.InputTokens += message.Usage.InputTokens
		usage.OutputTokens += message.Usage.OutputTokens
		usage.CacheReadTokens += message.Usage.CacheReadInputTokens
		usage.CacheWriteTokens += message.Usage.CacheCreationInputTokens

		logTurn(cfg.Log, usage.Turns, message)

		if message.StopReason == anthropic.BetaStopReasonRefusal {
			return nil, "", usage, fmt.Errorf("the model declined to review this diff (%s)", message.StopDetails.Category)
		}
		if sink.called {
			break
		}
	}

	usage.EstimatedUSDCents = estimateCents(cfg.Model, usage)

	if !sink.called {
		return nil, "", usage, errors.New("the reviewer never called report_findings — raise --max-turns or narrow the diff")
	}
	return sink.findings, sink.verdict, usage, nil
}

func logTurn(w io.Writer, turn int, m *anthropic.BetaMessage) {
	if w == nil {
		return
	}
	var calls []string
	for _, block := range m.Content {
		switch b := block.AsAny().(type) {
		case anthropic.BetaToolUseBlock:
			calls = append(calls, fmt.Sprintf("%s(%s)", b.Name, truncate(b.JSON.Input.Raw(), 120)))
		case anthropic.BetaTextBlock:
			if s := strings.TrimSpace(b.Text); s != "" {
				calls = append(calls, "say: "+truncate(s, 160))
			}
		}
	}
	fmt.Fprintf(w, "turn %d · %s\n", turn, strings.Join(calls, " · "))
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// price is dollars per million tokens, as published for the first-party API.
type price struct{ in, out float64 }

var prices = map[string]price{
	"claude-opus-5":    {5.00, 25.00},
	"claude-opus-4-8":  {5.00, 25.00},
	"claude-sonnet-5":  {2.00, 10.00},
	"claude-haiku-4-5": {1.00, 5.00},
	"claude-fable-5-1": {10.00, 50.00},
}

// estimateCents is a running cost figure for the PR comment. Cache reads bill at a
// tenth of the input rate and cache writes at 1.25x, which is the whole reason the
// rubric sits behind a cache breakpoint.
func estimateCents(model string, u gate.Usage) float64 {
	p, ok := prices[model]
	if !ok {
		return 0
	}
	const perToken = 1_000_000.0
	dollars := float64(u.InputTokens)*p.in/perToken +
		float64(u.OutputTokens)*p.out/perToken +
		float64(u.CacheReadTokens)*(p.in*0.1)/perToken +
		float64(u.CacheWriteTokens)*(p.in*1.25)/perToken
	return dollars * 100
}
