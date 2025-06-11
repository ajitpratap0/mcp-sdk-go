// Package main demonstrates JSON-RPC 2.0 batch processing functionality in the MCP Go SDK.
// This example shows how to create, serialize, parse, and process batch requests with proper JSON-RPC 2.0 compliance.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// ExampleToolsProvider implements tools for batch processing demonstration
type ExampleToolsProvider struct {
	requestCount int
}

func (p *ExampleToolsProvider) ListTools(ctx context.Context, category string, pagination *protocol.PaginationParams) ([]protocol.Tool, int, string, bool, error) {
	tools := []protocol.Tool{
		{
			Name:        "math.add",
			Description: "Add two numbers together",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {"a": {"type": "number"}, "b": {"type": "number"}}, "required": ["a", "b"]}`),
		},
		{
			Name:        "math.multiply",
			Description: "Multiply two numbers",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {"a": {"type": "number"}, "b": {"type": "number"}}, "required": ["a", "b"]}`),
		},
		{
			Name:        "string.concat",
			Description: "Concatenate two strings",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {"a": {"type": "string"}, "b": {"type": "string"}}, "required": ["a", "b"]}`),
		},
		{
			Name:        "time.now",
			Description: "Get current timestamp",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
	}

	return tools, len(tools), "", false, nil
}

func (p *ExampleToolsProvider) CallTool(ctx context.Context, name string, input json.RawMessage, contextData json.RawMessage) (*protocol.CallToolResult, error) {
	p.requestCount++

	var inputMap map[string]interface{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &inputMap); err != nil {
			return nil, fmt.Errorf("invalid input format: %w", err)
		}
	}

	switch name {
	case "math.add":
		a, aOk := inputMap["a"].(float64)
		b, bOk := inputMap["b"].(float64)
		if !aOk || !bOk {
			return nil, fmt.Errorf("math.add requires numeric inputs 'a' and 'b'")
		}
		result := a + b
		resultJSON, _ := json.Marshal(map[string]interface{}{
			"operation": "add",
			"operands":  []float64{a, b},
			"result":    result,
			"text":      fmt.Sprintf("Result: %.2f + %.2f = %.2f", a, b, result),
		})
		return &protocol.CallToolResult{
			Result: resultJSON,
		}, nil

	case "math.multiply":
		a, aOk := inputMap["a"].(float64)
		b, bOk := inputMap["b"].(float64)
		if !aOk || !bOk {
			return nil, fmt.Errorf("math.multiply requires numeric inputs 'a' and 'b'")
		}
		result := a * b
		resultJSON, _ := json.Marshal(map[string]interface{}{
			"operation": "multiply",
			"operands":  []float64{a, b},
			"result":    result,
			"text":      fmt.Sprintf("Result: %.2f × %.2f = %.2f", a, b, result),
		})
		return &protocol.CallToolResult{
			Result: resultJSON,
		}, nil

	case "string.concat":
		a, aOk := inputMap["a"].(string)
		b, bOk := inputMap["b"].(string)
		if !aOk || !bOk {
			return nil, fmt.Errorf("string.concat requires string inputs 'a' and 'b'")
		}
		result := a + b
		resultJSON, _ := json.Marshal(map[string]interface{}{
			"operation": "concat",
			"operands":  []string{a, b},
			"result":    result,
			"text":      fmt.Sprintf("Result: '%s' + '%s' = '%s'", a, b, result),
		})
		return &protocol.CallToolResult{
			Result: resultJSON,
		}, nil

	case "time.now":
		now := time.Now()
		resultJSON, _ := json.Marshal(map[string]interface{}{
			"operation": "time",
			"timestamp": now.Unix(),
			"formatted": now.Format(time.RFC3339),
			"text":      fmt.Sprintf("Current time: %s (timestamp: %d)", now.Format(time.RFC3339), now.Unix()),
		})
		return &protocol.CallToolResult{
			Result: resultJSON,
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func main() {
	fmt.Println("🚀 MCP Batch Processing Example")
	fmt.Println("This example demonstrates JSON-RPC 2.0 batch processing with the MCP Go SDK")

	// Demonstrate JSON-RPC batch structures
	demonstrateJSONRPCBatchTypes()

	// Demonstrate transport layer batch processing
	demonstrateTransportBatchProcessing()

	// Demonstrate various batch scenarios
	demonstrateBatchScenarios()

	fmt.Println("\n🎉 Batch processing demonstration completed!")
}

func demonstrateJSONRPCBatchTypes() {
	fmt.Println("\n📋 JSON-RPC 2.0 Batch Types Demonstration")

	// 1. Create individual requests and notifications
	mathReq, _ := protocol.NewRequest("req-1", "callTool", map[string]interface{}{
		"name":  "math.add",
		"input": map[string]float64{"a": 10, "b": 5},
	})

	stringReq, _ := protocol.NewRequest("req-2", "callTool", map[string]interface{}{
		"name":  "string.concat",
		"input": map[string]string{"a": "Hello", "b": " World"},
	})

	timeReq, _ := protocol.NewRequest("req-3", "callTool", map[string]interface{}{
		"name":  "time.now",
		"input": map[string]interface{}{},
	})

	// Create a notification (no response expected)
	statusNotif, _ := protocol.NewNotification("status", map[string]string{
		"message": "Processing batch request",
	})

	// 2. Create batch request
	batchRequest, err := protocol.NewJSONRPCBatchRequest(mathReq, stringReq, timeReq, statusNotif)
	if err != nil {
		log.Fatalf("Failed to create batch request: %v", err)
	}

	fmt.Printf("   📦 Created batch with %d items\n", batchRequest.Len())
	fmt.Printf("   📝 Requests: %d\n", len(batchRequest.GetRequests()))
	fmt.Printf("   🔔 Notifications: %d\n", len(batchRequest.GetNotifications()))

	// 3. Serialize to JSON
	batchJSON, err := batchRequest.ToJSON()
	if err != nil {
		log.Fatalf("Failed to serialize batch: %v", err)
	}

	fmt.Printf("   📄 Batch JSON (first 200 chars): %s...\n", string(batchJSON)[:min(len(batchJSON), 200)])

	// 4. Parse batch back from JSON
	parsedBatch, err := protocol.ParseJSONRPCBatchRequest(batchJSON)
	if err != nil {
		log.Fatalf("Failed to parse batch: %v", err)
	}

	fmt.Printf("   ✅ Successfully parsed batch with %d items\n", parsedBatch.Len())

	// 5. Create sample batch response
	resp1, _ := protocol.NewResponse("req-1", map[string]interface{}{
		"result": "15.00",
	})
	resp2, _ := protocol.NewResponse("req-2", map[string]interface{}{
		"result": "Hello World",
	})
	resp3, _ := protocol.NewResponse("req-3", map[string]interface{}{
		"result": "2025-01-01T12:00:00Z",
	})

	batchResponse := protocol.NewJSONRPCBatchResponse(resp1, resp2, resp3)
	fmt.Printf("   📋 Created batch response with %d responses\n", batchResponse.Len())

	responseJSON, _ := batchResponse.ToJSON()
	fmt.Printf("   📄 Response JSON (first 150 chars): %s...\n", string(responseJSON)[:min(len(responseJSON), 150)])
}

func demonstrateTransportBatchProcessing() {
	fmt.Println("\n🔄 Transport Layer Batch Processing Demonstration")

	// Create a transport with the tools provider for testing
	transport := transport.NewBaseTransport()
	toolsProvider := &ExampleToolsProvider{}

	// Register tool handler
	transport.RegisterRequestHandler("callTool", func(ctx context.Context, params interface{}) (interface{}, error) {
		// Parse parameters - handle both json.RawMessage and map[string]interface{}
		var paramMap map[string]interface{}

		switch p := params.(type) {
		case json.RawMessage:
			if err := json.Unmarshal(p, &paramMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal parameters: %w", err)
			}
		case map[string]interface{}:
			paramMap = p
		default:
			return nil, fmt.Errorf("invalid parameter type: %T", params)
		}

		toolName, ok := paramMap["name"].(string)
		if !ok {
			return nil, fmt.Errorf("missing tool name")
		}

		inputData, _ := json.Marshal(paramMap["input"])
		return toolsProvider.CallTool(ctx, toolName, inputData, nil)
	})

	ctx := context.Background()

	// Scenario 1: Process a simple batch
	fmt.Println("\n1️⃣  Processing Simple Math Batch")
	mathBatch := createMathBatch()
	response, err := transport.HandleBatchRequest(ctx, mathBatch)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ Processed %d requests, got %d responses\n", mathBatch.Len(), response.Len())
		for i, resp := range *response {
			fmt.Printf("      Response %d (ID: %v): %s\n", i+1, resp.ID, getResponseSummary(resp))
		}
	}

	// Scenario 2: Process batch with notifications
	fmt.Println("\n2️⃣  Processing Batch with Notifications")
	notificationBatch := createNotificationBatch()
	response, err = transport.HandleBatchRequest(ctx, notificationBatch)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else {
		fmt.Printf("   ✅ Processed %d items (%d requests, %d notifications)\n",
			notificationBatch.Len(), len(notificationBatch.GetRequests()), len(notificationBatch.GetNotifications()))
		fmt.Printf("   📋 Got %d responses (notifications don't generate responses)\n", response.Len())
	}

	fmt.Printf("\n📊 Total tool calls processed: %d\n", toolsProvider.requestCount)
}

func createMathBatch() *protocol.JSONRPCBatchRequest {
	req1, _ := protocol.NewRequest("math-1", "callTool", map[string]interface{}{
		"name":  "math.add",
		"input": map[string]float64{"a": 15, "b": 25},
	})

	req2, _ := protocol.NewRequest("math-2", "callTool", map[string]interface{}{
		"name":  "math.multiply",
		"input": map[string]float64{"a": 7, "b": 8},
	})

	req3, _ := protocol.NewRequest("math-3", "callTool", map[string]interface{}{
		"name":  "math.add",
		"input": map[string]float64{"a": 100, "b": 200},
	})

	batch, _ := protocol.NewJSONRPCBatchRequest(req1, req2, req3)
	return batch
}

func createMixedBatch() *protocol.JSONRPCBatchRequest {
	req1, _ := protocol.NewRequest("mixed-1", "callTool", map[string]interface{}{
		"name":  "string.concat",
		"input": map[string]string{"a": "Batch", "b": "Processing"},
	})

	req2, _ := protocol.NewRequest("mixed-2", "callTool", map[string]interface{}{
		"name":  "time.now",
		"input": map[string]interface{}{},
	})

	req3, _ := protocol.NewRequest("mixed-3", "callTool", map[string]interface{}{
		"name":  "math.multiply",
		"input": map[string]float64{"a": 3.14, "b": 2},
	})

	batch, _ := protocol.NewJSONRPCBatchRequest(req1, req2, req3)
	return batch
}

func createNotificationBatch() *protocol.JSONRPCBatchRequest {
	req1, _ := protocol.NewRequest("notif-1", "callTool", map[string]interface{}{
		"name":  "time.now",
		"input": map[string]interface{}{},
	})

	// Add notifications (these won't generate responses)
	notif1, _ := protocol.NewNotification("log", map[string]string{
		"level":   "info",
		"message": "Processing batch with notifications",
	})

	notif2, _ := protocol.NewNotification("metrics", map[string]interface{}{
		"event": "batch_request",
		"size":  3,
	})

	req2, _ := protocol.NewRequest("notif-2", "callTool", map[string]interface{}{
		"name":  "string.concat",
		"input": map[string]string{"a": "Notification", "b": "Test"},
	})

	batch, _ := protocol.NewJSONRPCBatchRequest(req1, notif1, notif2, req2)
	return batch
}

func createErrorBatch() *protocol.JSONRPCBatchRequest {
	// Valid request
	req1, _ := protocol.NewRequest("error-1", "callTool", map[string]interface{}{
		"name":  "math.add",
		"input": map[string]float64{"a": 5, "b": 10},
	})

	// Invalid tool name (will cause error)
	req2, _ := protocol.NewRequest("error-2", "callTool", map[string]interface{}{
		"name":  "invalid.tool",
		"input": map[string]interface{}{},
	})

	// Invalid parameters (will cause error)
	req3, _ := protocol.NewRequest("error-3", "callTool", map[string]interface{}{
		"name":  "math.add",
		"input": map[string]string{"a": "not_a_number", "b": "also_not_a_number"},
	})

	// Another valid request
	req4, _ := protocol.NewRequest("error-4", "callTool", map[string]interface{}{
		"name":  "time.now",
		"input": map[string]interface{}{},
	})

	batch, _ := protocol.NewJSONRPCBatchRequest(req1, req2, req3, req4)
	return batch
}

func createLargeBatch() *protocol.JSONRPCBatchRequest {
	requests := make([]interface{}, 50)

	for i := 0; i < 50; i++ {
		req, _ := protocol.NewRequest(fmt.Sprintf("large-%d", i), "callTool", map[string]interface{}{
			"name":  "math.add",
			"input": map[string]float64{"a": float64(i), "b": float64(i * 2)},
		})
		requests[i] = req
	}

	batch, _ := protocol.NewJSONRPCBatchRequest(requests...)
	return batch
}

func demonstrateBatchScenarios() {
	fmt.Println("\n📊 Batch Processing Scenarios")

	// Scenario 1: Serialization roundtrip test
	fmt.Println("\n1️⃣  Serialization Roundtrip Test")
	mixedBatch := createMixedBatch()

	// Serialize to JSON
	jsonData, err := mixedBatch.ToJSON()
	if err != nil {
		log.Fatalf("Failed to serialize batch: %v", err)
	}
	fmt.Printf("   📄 Serialized batch: %d bytes\n", len(jsonData))

	// Parse back from JSON
	parsedBatch, err := protocol.ParseJSONRPCBatchRequest(jsonData)
	if err != nil {
		log.Fatalf("Failed to parse batch: %v", err)
	}
	fmt.Printf("   ✅ Successfully parsed %d items back from JSON\n", parsedBatch.Len())

	// Scenario 2: Large batch performance test
	fmt.Println("\n2️⃣  Large Batch Performance Test")
	largeBatch := createLargeBatch()

	start := time.Now()
	serialized, _ := largeBatch.ToJSON()
	serializeTime := time.Since(start)

	start = time.Now()
	parsed, _ := protocol.ParseJSONRPCBatchRequest(serialized)
	parseTime := time.Since(start)

	fmt.Printf("   📦 Batch size: %d requests\n", largeBatch.Len())
	fmt.Printf("   📄 Serialized size: %d bytes\n", len(serialized))
	fmt.Printf("   ⏱️  Serialization time: %v\n", serializeTime)
	fmt.Printf("   ⏱️  Parsing time: %v\n", parseTime)
	fmt.Printf("   ✅ Parsed back %d requests successfully\n", parsed.Len())
	fmt.Printf("   🚀 Serialization throughput: %.1f req/ms\n", float64(largeBatch.Len())/float64(serializeTime.Nanoseconds()/1e6))

	// Scenario 3: Error scenario handling
	fmt.Println("\n3️⃣  Error Handling Test")
	errorBatch := createErrorBatch()

	fmt.Printf("   📦 Created error test batch with %d requests\n", errorBatch.Len())
	fmt.Printf("   📝 Request breakdown:\n")
	for i, req := range errorBatch.GetRequests() {
		var params map[string]interface{}
		json.Unmarshal(req.Params, &params)
		toolName := params["name"].(string)
		if toolName == "invalid.tool" {
			fmt.Printf("      %d. Invalid tool (will cause error)\n", i+1)
		} else {
			fmt.Printf("      %d. Valid tool: %s\n", i+1, toolName)
		}
	}
}

func getResponseSummary(resp *protocol.Response) string {
	if resp.Error != nil {
		return fmt.Sprintf("Error: %s", resp.Error.Message)
	}
	if resp.Result != nil {
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Result, &result); err == nil {
			if text, ok := result["text"].(string); ok {
				if len(text) > 40 {
					return text[:40] + "..."
				}
				return text
			}
		}
		return "Success (result data)"
	}
	return "Success (no result)"
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
