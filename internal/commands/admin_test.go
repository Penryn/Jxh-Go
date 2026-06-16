package commands_test

import (
	"context"
	"testing"

	"github.com/zjutjh/jxh-go/internal/commands"
)

func TestAdminAddAndList(t *testing.T) {
	store := commands.NewMemoryAdminStore()
	handler := commands.NewAdminHandler(store)
	got, err := handler.Handle(context.Background(), commands.AdminInput{
		ActorID: 1,
		Text:    "添加管理员",
		AtUsers: []int64{2},
		IsOwner: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("empty response")
	}
	list, err := handler.Handle(context.Background(), commands.AdminInput{ActorID: 1, Text: "所有管理员", IsOwner: true})
	if err != nil {
		t.Fatal(err)
	}
	if list != "当前管理员：2" {
		t.Fatalf("list = %q", list)
	}
}
