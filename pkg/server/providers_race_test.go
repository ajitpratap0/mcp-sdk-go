package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// TestBaseProvidersRaceSafety tests that all base providers are race-safe
func TestBaseProvidersRaceSafety(t *testing.T) {
	t.Run("BaseToolsProvider", func(t *testing.T) {
		provider := NewBaseToolsProvider()
		var wg sync.WaitGroup

		// Test concurrent registration and listing
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				tool := protocol.Tool{
					Name:        string(rune('A' + id%26)),
					Description: "Test tool",
				}
				provider.RegisterTool(tool)
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, _, _, _ = provider.ListTools(context.Background(), "", nil)
			}()
		}

		wg.Wait()
	})

	t.Run("BaseResourcesProvider", func(t *testing.T) {
		provider := NewBaseResourcesProvider()
		var wg sync.WaitGroup

		// Test concurrent registration
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				if id%2 == 0 {
					resource := protocol.Resource{
						URI:  string(rune('A' + id%26)),
						Name: "Test resource",
					}
					provider.RegisterResource(resource)
				} else {
					template := protocol.ResourceTemplate{
						URI:  string(rune('A' + id%26)),
						Name: "Test template",
					}
					provider.RegisterTemplate(template)
				}
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, _, _, _, _ = provider.ListResources(context.Background(), "", false, nil)
			}()
		}

		wg.Wait()
	})

	t.Run("BasePromptsProvider", func(t *testing.T) {
		provider := NewBasePromptsProvider()
		var wg sync.WaitGroup

		// Test concurrent registration and operations
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				prompt := protocol.Prompt{
					ID:          string(rune('A' + id%26)),
					Name:        "Test prompt",
					Description: "Test description",
				}
				provider.RegisterPrompt(prompt)
			}(i)
		}

		// Concurrent reads - list and get
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				if id%2 == 0 {
					_, _, _, _, _ = provider.ListPrompts(context.Background(), "", nil)
				} else {
					_, _ = provider.GetPrompt(context.Background(), string(rune('A'+id%26)))
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("BaseRootsProvider", func(t *testing.T) {
		provider := NewBaseRootsProvider()
		var wg sync.WaitGroup

		// Test concurrent registration and listing
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				root := protocol.Root{
					ID:           string(rune('A' + id%26)),
					Name:         "Test root",
					ResourceURIs: []string{"/test/uri"},
				}
				provider.RegisterRoot(root)
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, _, _, _ = provider.ListRoots(context.Background(), "", nil)
			}()
		}

		wg.Wait()
	})
}

// TestBaseProvidersStressTest performs a stress test with mixed read/write operations
func TestBaseProvidersStressTest(t *testing.T) {
	toolsProvider := NewBaseToolsProvider()
	resourcesProvider := NewBaseResourcesProvider()
	promptsProvider := NewBasePromptsProvider()
	rootsProvider := NewBaseRootsProvider()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Perform mixed operations for 2 seconds
	stopCh := make(chan struct{})
	go func() {
		time.Sleep(2 * time.Second)
		close(stopCh)
	}()

	// Tools provider stress test
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			count := 0
			for {
				select {
				case <-stopCh:
					t.Logf("Tools worker %d performed %d operations", worker, count)
					return
				default:
					if count%3 == 0 {
						tool := protocol.Tool{
							Name: "tool-" + string(rune('A'+count%26)),
						}
						toolsProvider.RegisterTool(tool)
					} else {
						toolsProvider.ListTools(ctx, "", nil)
					}
					count++
				}
			}
		}(i)
	}

	// Resources provider stress test
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			count := 0
			for {
				select {
				case <-stopCh:
					t.Logf("Resources worker %d performed %d operations", worker, count)
					return
				default:
					switch count % 4 {
					case 0:
						resource := protocol.Resource{
							URI: "resource-" + string(rune('A'+count%26)),
						}
						resourcesProvider.RegisterResource(resource)
					case 1:
						template := protocol.ResourceTemplate{
							URI: "template-" + string(rune('A'+count%26)),
						}
						resourcesProvider.RegisterTemplate(template)
					default:
						resourcesProvider.ListResources(ctx, "", false, nil)
					}
					count++
				}
			}
		}(i)
	}

	// Prompts provider stress test
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			count := 0
			for {
				select {
				case <-stopCh:
					t.Logf("Prompts worker %d performed %d operations", worker, count)
					return
				default:
					switch count % 4 {
					case 0:
						prompt := protocol.Prompt{
							ID: "prompt-" + string(rune('A'+count%26)),
						}
						promptsProvider.RegisterPrompt(prompt)
					case 1:
						promptsProvider.GetPrompt(ctx, "prompt-"+string(rune('A'+count%26)))
					default:
						promptsProvider.ListPrompts(ctx, "", nil)
					}
					count++
				}
			}
		}(i)
	}

	// Roots provider stress test
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			count := 0
			for {
				select {
				case <-stopCh:
					t.Logf("Roots worker %d performed %d operations", worker, count)
					return
				default:
					if count%3 == 0 {
						root := protocol.Root{
							ID:           "root-" + string(rune('A'+count%26)),
							Name:         "Test root",
							ResourceURIs: []string{"/test/uri"},
						}
						rootsProvider.RegisterRoot(root)
					} else {
						rootsProvider.ListRoots(ctx, "", nil)
					}
					count++
				}
			}
		}(i)
	}

	wg.Wait()
}
