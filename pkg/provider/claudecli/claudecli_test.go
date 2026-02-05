package claudecli

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/xinguang/agentic-coder/pkg/provider"
)

func TestStreamReaderToolEvents(t *testing.T) {
	// Simulate Claude CLI JSON output with tool_use and tool_result
	cliOutput := `{"type":"assistant","message":{"content":[{"type":"text","text":"Let me read the file."}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"Let me read the file."},{"type":"tool_use","id":"tool_1","name":"Read","input":{"file_path":"/test.txt"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"file contents here","is_error":false}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"The file contains..."}]}}
{"type":"result","result":"completed"}
`

	// Create a stream reader with the simulated output
	reader := strings.NewReader(cliOutput)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	sr := &streamReader{
		scanner: scanner,
		done:    false,
	}

	// Collect all events
	var events []provider.StreamingEvent
	for {
		event, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error: %v", err)
		}
		events = append(events, event)
	}

	// Verify we got expected event types
	var hasMessageStart, hasTextDelta, hasToolInfo, hasToolResult, hasMessageStop bool
	for _, ev := range events {
		switch ev.(type) {
		case *provider.MessageStartEvent:
			hasMessageStart = true
		case *provider.ContentBlockDeltaEvent:
			hasTextDelta = true
		case *provider.ToolInfoEvent:
			hasToolInfo = true
			toolEv := ev.(*provider.ToolInfoEvent)
			if toolEv.Name != "Read" {
				t.Errorf("Expected tool name 'Read', got '%s'", toolEv.Name)
			}
			if toolEv.ID != "tool_1" {
				t.Errorf("Expected tool ID 'tool_1', got '%s'", toolEv.ID)
			}
		case *provider.ToolResultInfoEvent:
			hasToolResult = true
			resultEv := ev.(*provider.ToolResultInfoEvent)
			if resultEv.Content != "file contents here" {
				t.Errorf("Expected content 'file contents here', got '%s'", resultEv.Content)
			}
		case *provider.MessageStopEvent:
			hasMessageStop = true
		}
	}

	if !hasMessageStart {
		t.Error("Missing MessageStartEvent")
	}
	if !hasTextDelta {
		t.Error("Missing ContentBlockDeltaEvent (text)")
	}
	if !hasToolInfo {
		t.Error("Missing ToolInfoEvent")
	}
	if !hasToolResult {
		t.Error("Missing ToolResultInfoEvent")
	}
	if !hasMessageStop {
		t.Error("Missing MessageStopEvent")
	}

	t.Logf("Received %d events total", len(events))
}
