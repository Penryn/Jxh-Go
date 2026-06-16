package commands

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type AdminInput struct {
	ActorID int64
	Text    string
	AtUsers []int64
	IsOwner bool
}

type AdminStore interface {
	AddAdmin(ctx context.Context, userID int64) error
	RemoveAdmin(ctx context.Context, userID int64) error
	ClearAdmins(ctx context.Context) error
	ListAdmins(ctx context.Context) ([]int64, error)
	AddBlacklist(ctx context.Context, userID int64) error
	RemoveBlacklist(ctx context.Context, userID int64) error
	ClearBlacklist(ctx context.Context) error
	ListBlacklist(ctx context.Context) ([]int64, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
}

type SchedulerStore interface {
	ListScheduledJobs(ctx context.Context) ([]ScheduledJobView, error)
	AddScheduledJob(ctx context.Context, job ScheduledJobInput) (uint64, error)
	RemoveScheduledJob(ctx context.Context, id uint64) error
}

type ScheduledJobInput struct {
	Type     string
	TimeHHMM string
	GroupID  int64
	Message  string
}

type ScheduledJobView struct {
	ID       uint64
	Type     string
	TimeHHMM string
	GroupID  int64
	Message  string
	Enabled  bool
}

type AdminHandler struct {
	store AdminStore
}

func NewAdminHandler(store AdminStore) *AdminHandler {
	return &AdminHandler{store: store}
}

func (h *AdminHandler) Handle(ctx context.Context, input AdminInput) (string, error) {
	if h.store == nil {
		return "管理员存储未初始化", nil
	}
	if !input.IsOwner {
		ok, err := h.store.IsAdmin(ctx, input.ActorID)
		if err != nil {
			return "", err
		}
		if !ok {
			return "~你好像没有权限执行该项操作耶~", nil
		}
	}
	text := strings.TrimSpace(input.Text)
	switch {
	case text == "添加管理员":
		userID, ok := firstAt(input.AtUsers)
		if !ok {
			return "请 @ 要添加的管理员", nil
		}
		return "已添加管理员", h.store.AddAdmin(ctx, userID)
	case text == "移除管理员":
		userID, ok := firstAt(input.AtUsers)
		if !ok {
			return "请 @ 要移除的管理员", nil
		}
		return "已移除管理员", h.store.RemoveAdmin(ctx, userID)
	case text == "移除所有管理员":
		return "已移除所有管理员", h.store.ClearAdmins(ctx)
	case text == "所有管理员":
		users, err := h.store.ListAdmins(ctx)
		if err != nil {
			return "", err
		}
		return "当前管理员：" + joinIDs(users), nil
	case text == "添加黑名单":
		userID, ok := firstAt(input.AtUsers)
		if !ok {
			return "请 @ 要添加的黑名单用户", nil
		}
		return "已添加黑名单", h.store.AddBlacklist(ctx, userID)
	case text == "移除黑名单":
		userID, ok := firstAt(input.AtUsers)
		if !ok {
			return "请 @ 要移除的黑名单用户", nil
		}
		return "已移除黑名单", h.store.RemoveBlacklist(ctx, userID)
	case text == "移除所有黑名单":
		return "已移除所有黑名单", h.store.ClearBlacklist(ctx)
	case text == "所有黑名单":
		users, err := h.store.ListBlacklist(ctx)
		if err != nil {
			return "", err
		}
		return "当前黑名单：" + joinIDs(users), nil
	case text == "定时任务 查看":
		scheduler, ok := h.store.(SchedulerStore)
		if !ok {
			return "定时任务存储未初始化", nil
		}
		jobs, err := scheduler.ListScheduledJobs(ctx)
		if err != nil {
			return "", err
		}
		if len(jobs) == 0 {
			return "~当前没有定时任务~", nil
		}
		lines := []string{"当前定时任务列表:"}
		for _, job := range jobs {
			lines = append(lines, fmt.Sprintf("%d. %s %s 群:%d %s", job.ID, job.Type, job.TimeHHMM, job.GroupID, job.Message))
		}
		return strings.Join(lines, "\n"), nil
	case strings.HasPrefix(text, "定时任务 添加 "):
		scheduler, ok := h.store.(SchedulerStore)
		if !ok {
			return "定时任务存储未初始化", nil
		}
		parts := strings.SplitN(strings.TrimPrefix(text, "定时任务 添加 "), " ", 4)
		if len(parts) < 4 {
			return "格式：/admin 定时任务 添加 <每天|单次> <时间> <群聊ID> <消息内容>", nil
		}
		groupID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return "群聊ID格式不正确", nil
		}
		id, err := scheduler.AddScheduledJob(ctx, ScheduledJobInput{Type: parts[0], TimeHHMM: parts[1], GroupID: groupID, Message: parts[3]})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("已添加定时任务 %d", id), nil
	case strings.HasPrefix(text, "定时任务 移除 "):
		scheduler, ok := h.store.(SchedulerStore)
		if !ok {
			return "定时任务存储未初始化", nil
		}
		id, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(text, "定时任务 移除 ")), 10, 64)
		if err != nil {
			return "任务编号格式不正确", nil
		}
		return "已移除定时任务", scheduler.RemoveScheduledJob(ctx, id)
	default:
		return "未知管理命令", nil
	}
}

func firstAt(users []int64) (int64, bool) {
	if len(users) == 0 {
		return 0, false
	}
	return users[0], true
}

func joinIDs(users []int64) string {
	if len(users) == 0 {
		return "无"
	}
	sort.Slice(users, func(i, j int) bool { return users[i] < users[j] })
	parts := make([]string, len(users))
	for i, user := range users {
		parts[i] = fmt.Sprintf("%d", user)
	}
	return strings.Join(parts, "、")
}

type MemoryAdminStore struct {
	mu        sync.Mutex
	admins    map[int64]struct{}
	blacklist map[int64]struct{}
}

func NewMemoryAdminStore() *MemoryAdminStore {
	return &MemoryAdminStore{admins: map[int64]struct{}{}, blacklist: map[int64]struct{}{}}
}

func (s *MemoryAdminStore) AddAdmin(ctx context.Context, userID int64) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.admins[userID] = struct{}{}
	return nil
}

func (s *MemoryAdminStore) RemoveAdmin(ctx context.Context, userID int64) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.admins, userID)
	return nil
}

func (s *MemoryAdminStore) ClearAdmins(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.admins = map[int64]struct{}{}
	return nil
}

func (s *MemoryAdminStore) ListAdmins(ctx context.Context) ([]int64, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int64, 0, len(s.admins))
	for user := range s.admins {
		out = append(out, user)
	}
	return out, nil
}

func (s *MemoryAdminStore) AddBlacklist(ctx context.Context, userID int64) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blacklist[userID] = struct{}{}
	return nil
}

func (s *MemoryAdminStore) RemoveBlacklist(ctx context.Context, userID int64) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.blacklist, userID)
	return nil
}

func (s *MemoryAdminStore) ClearBlacklist(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blacklist = map[int64]struct{}{}
	return nil
}

func (s *MemoryAdminStore) ListBlacklist(ctx context.Context) ([]int64, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int64, 0, len(s.blacklist))
	for user := range s.blacklist {
		out = append(out, user)
	}
	return out, nil
}

func (s *MemoryAdminStore) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.admins[userID]
	return ok, nil
}

func (s *MemoryAdminStore) ListScheduledJobs(ctx context.Context) ([]ScheduledJobView, error) {
	_ = ctx
	return nil, nil
}

func (s *MemoryAdminStore) AddScheduledJob(ctx context.Context, job ScheduledJobInput) (uint64, error) {
	_ = ctx
	_ = job
	return 1, nil
}

func (s *MemoryAdminStore) RemoveScheduledJob(ctx context.Context, id uint64) error {
	_ = ctx
	_ = id
	return nil
}
