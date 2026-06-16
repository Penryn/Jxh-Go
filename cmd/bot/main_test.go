package main

import (
	"testing"

	"github.com/zjutjh/jxh-go/internal/config"
)

func TestShouldCreateEinoChatAllowsArkDefaultBaseURL(t *testing.T) {
	if !shouldCreateEinoChat(config.AIConfig{
		Provider: "ark",
		APIKey:   "key",
		Model:    "ep-20260617000000-demo",
	}) {
		t.Fatal("expected ark config with default base url to create eino chat")
	}
}

func TestShouldCreateEinoChatRequiresCredentials(t *testing.T) {
	if shouldCreateEinoChat(config.AIConfig{
		Provider: "ark",
		Model:    "ep-20260617000000-demo",
	}) {
		t.Fatal("expected missing credentials to skip eino chat")
	}
}
