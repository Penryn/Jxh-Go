package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
)

const EmptyKnowledgeAnswer = "知识库里没有找到相关内容"

type Document struct {
	ID       string
	Content  string
	Metadata map[string]string
	Score    float64
}

type Retriever interface {
	Retrieve(ctx context.Context, query string, topK int) ([]Document, error)
}

type RetrieverRef struct {
	value atomic.Value
}

func NewRetrieverRef(initial Retriever) *RetrieverRef {
	ref := &RetrieverRef{}
	if initial != nil {
		ref.Set(initial)
	}
	return ref
}

func (r *RetrieverRef) Set(next Retriever) {
	r.value.Store(next)
}

func (r *RetrieverRef) Retrieve(ctx context.Context, query string, topK int) ([]Document, error) {
	if r == nil {
		return nil, nil
	}
	value := r.value.Load()
	if value == nil {
		return nil, nil
	}
	return value.(Retriever).Retrieve(ctx, query, topK)
}

type Chat interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type Options struct {
	Retriever        Retriever
	Chat             Chat
	TopK             int
	MaxQuestionChars int
}

type Service struct {
	retriever        Retriever
	chat             Chat
	topK             int
	maxQuestionChars int
}

func NewService(opts Options) *Service {
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}
	maxQuestionChars := opts.MaxQuestionChars
	if maxQuestionChars <= 0 {
		maxQuestionChars = 500
	}
	return &Service{
		retriever:        opts.Retriever,
		chat:             opts.Chat,
		topK:             topK,
		maxQuestionChars: maxQuestionChars,
	}
}

func (s *Service) Answer(ctx context.Context, question string) (string, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return EmptyKnowledgeAnswer, nil
	}
	if len([]rune(question)) > s.maxQuestionChars {
		question = string([]rune(question)[:s.maxQuestionChars])
	}
	if s.retriever == nil {
		return EmptyKnowledgeAnswer, nil
	}
	docs, err := s.retriever.Retrieve(ctx, question, s.topK)
	if err != nil {
		return "", err
	}
	if len(docs) == 0 {
		return EmptyKnowledgeAnswer, nil
	}
	prompt := BuildPrompt(question, docs)
	if s.chat == nil {
		return EmptyKnowledgeAnswer, nil
	}
	answer, err := s.chat.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return EmptyKnowledgeAnswer, nil
	}
	return answer, nil
}

func BuildPrompt(question string, docs []Document) string {
	var b strings.Builder
	b.WriteString("你是精小弘。只能基于以下知识库内容回答用户问题；如果知识库没有答案，回答“")
	b.WriteString(EmptyKnowledgeAnswer)
	b.WriteString("”。不要编造学校政策、流程、时间、联系方式。\n\n")
	b.WriteString("用户问题：")
	b.WriteString(question)
	b.WriteString("\n\n知识库内容：\n")
	for i, doc := range docs {
		b.WriteString(fmt.Sprintf("[%d] ID=%s\n", i+1, doc.ID))
		if len(doc.Metadata) > 0 {
			keys := make([]string, 0, len(doc.Metadata))
			for key := range doc.Metadata {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				b.WriteString(key)
				b.WriteString(": ")
				b.WriteString(doc.Metadata[key])
				b.WriteString("\n")
			}
		}
		b.WriteString(doc.Content)
		b.WriteString("\n\n")
	}
	return b.String()
}

type StaticRetriever struct {
	Documents []Document
}

func (r StaticRetriever) Retrieve(ctx context.Context, query string, topK int) ([]Document, error) {
	_ = ctx
	_ = query
	if topK > 0 && len(r.Documents) > topK {
		return r.Documents[:topK], nil
	}
	return r.Documents, nil
}

type StaticChat struct {
	Response   string
	LastPrompt string
}

func (c *StaticChat) Generate(ctx context.Context, prompt string) (string, error) {
	_ = ctx
	c.LastPrompt = prompt
	if c.Response == "" {
		return EmptyKnowledgeAnswer, nil
	}
	return c.Response, nil
}

type ExtractiveChat struct{}

func (ExtractiveChat) Generate(ctx context.Context, prompt string) (string, error) {
	_ = ctx
	const marker = "answer: "
	if idx := strings.Index(prompt, marker); idx >= 0 {
		rest := prompt[idx+len(marker):]
		if end := strings.Index(rest, "\n"); end >= 0 {
			rest = rest[:end]
		}
		if answer := strings.TrimSpace(rest); answer != "" {
			return answer, nil
		}
	}
	const body = "知识正文："
	if idx := strings.Index(prompt, body); idx >= 0 {
		answer := strings.TrimSpace(prompt[idx+len(body):])
		if answer != "" {
			if end := strings.Index(answer, "\n\n"); end >= 0 {
				answer = answer[:end]
			}
			return strings.TrimSpace(answer), nil
		}
	}
	return EmptyKnowledgeAnswer, nil
}
