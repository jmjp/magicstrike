package entities

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
)

func TestNewChatSession(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		matchIDs   []string
		question   string
		answer     string
		source     string
		dataPoints []DataPoint
		ttlDays    int
		wantErr    error
	}{
		{
			name:       "valid session created",
			userID:     "user_123",
			matchIDs:   []string{"match_abc"},
			question:   "What happened?",
			answer:     "Something happened.",
			source:     "clickhouse",
			dataPoints: []DataPoint{{Label: "key", Value: "val"}},
			ttlDays:    7,
			wantErr:    nil,
		},
		{
			name:       "empty user ID",
			userID:     "",
			matchIDs:   []string{"match_abc"},
			question:   "What happened?",
			answer:     "Something happened.",
			source:     "clickhouse",
			ttlDays:    7,
			wantErr:    ErrSessionUserIDRequired,
		},
		{
			name:       "no match IDs",
			userID:     "user_123",
			matchIDs:   nil,
			question:   "What happened?",
			answer:     "Something happened.",
			source:     "clickhouse",
			ttlDays:    7,
			wantErr:    ErrSessionNoMatches,
		},
		{
			name:       "empty match IDs",
			userID:     "user_123",
			matchIDs:   []string{},
			question:   "What happened?",
			answer:     "Something happened.",
			source:     "clickhouse",
			ttlDays:    7,
			wantErr:    ErrSessionNoMatches,
		},
		{
			name:       "empty question",
			userID:     "user_123",
			matchIDs:   []string{"match_abc"},
			question:   "",
			answer:     "Something happened.",
			source:     "clickhouse",
			ttlDays:    7,
			wantErr:    ErrSessionQuestionRequired,
		},
		{
			name:       "question too long",
			userID:     "user_123",
			matchIDs:   []string{"match_abc"},
			question:   strings.Repeat("a", 501),
			answer:     "Something happened.",
			source:     "clickhouse",
			ttlDays:    7,
			wantErr:    ErrSessionQuestionTooLong,
		},
		{
			name:       "empty answer",
			userID:     "user_123",
			matchIDs:   []string{"match_abc"},
			question:   "What happened?",
			answer:     "",
			source:     "clickhouse",
			ttlDays:    7,
			wantErr:    ErrSessionAnswerRequired,
		},
		{
			name:       "invalid source",
			userID:     "user_123",
			matchIDs:   []string{"match_abc"},
			question:   "What happened?",
			answer:     "Something happened.",
			source:     "invalid_source",
			ttlDays:    7,
			wantErr:    ErrInvalidSource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewChatSession(tt.userID, tt.matchIDs, tt.question, tt.answer, tt.source, tt.dataPoints, tt.ttlDays)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewChatSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil {
				if got == nil {
					t.Error("NewChatSession() returned nil for valid inputs")
					return
				}
			}
		})
	}
}

func TestNewChatSession_SuccessDetails(t *testing.T) {
	now := time.Now()
	ttlDays := 7

	s, err := NewChatSession(
		"user_123",
		[]string{"match_abc", "match_def"},
		"What happened?",
		"Something happened.",
		"clickhouse",
		[]DataPoint{{Label: "player", Value: "FalleN"}, {Label: "kills", Value: "32"}},
		ttlDays,
	)
	if err != nil {
		t.Fatalf("NewChatSession() unexpected error: %v", err)
	}

	// Verify ULID generation
	parsed, err := ulid.Parse(s.ID)
	if err != nil {
		t.Errorf("ID is not a valid ULID: %v", err)
	}
	_ = parsed

	// Verify timestamps are within reasonable range of now
	if s.CreatedAt.Before(now.Add(-time.Second)) || s.CreatedAt.After(now.Add(time.Second)) {
		t.Errorf("CreatedAt out of range: got %v, expected ~%v", s.CreatedAt, now)
	}
	if s.UpdatedAt.Before(now.Add(-time.Second)) || s.UpdatedAt.After(now.Add(time.Second)) {
		t.Errorf("UpdatedAt out of range: got %v, expected ~%v", s.UpdatedAt, now)
	}

	// Verify ExpiresAt = now + ttlDays * 24h
	expectedExpiry := now.Add(time.Duration(ttlDays) * 24 * time.Hour)
	diff := s.ExpiresAt.Sub(expectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("ExpiresAt out of range: got %v, expected ~%v", s.ExpiresAt, expectedExpiry)
	}

	// Verify initial message
	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.Messages))
	}
	if s.Messages[0].Question != "What happened?" {
		t.Errorf("unexpected question: got %q", s.Messages[0].Question)
	}
	if s.Messages[0].Answer != "Something happened." {
		t.Errorf("unexpected answer: got %q", s.Messages[0].Answer)
	}
	if s.Messages[0].Source != "clickhouse" {
		t.Errorf("unexpected source: got %q", s.Messages[0].Source)
	}
	if len(s.Messages[0].DataPoints) != 2 {
		t.Fatalf("expected 2 data points, got %d", len(s.Messages[0].DataPoints))
	}
	if s.Messages[0].DataPoints[0].Label != "player" || s.Messages[0].DataPoints[0].Value != "FalleN" {
		t.Errorf("unexpected first data point: got {%s, %s}", s.Messages[0].DataPoints[0].Label, s.Messages[0].DataPoints[0].Value)
	}
	if s.Messages[0].CreatedAt.Before(now.Add(-time.Second)) || s.Messages[0].CreatedAt.After(now.Add(time.Second)) {
		t.Errorf("Message.CreatedAt out of range: got %v", s.Messages[0].CreatedAt)
	}
}

func TestNewChatSession_ULIDUniqueness(t *testing.T) {
	s1, err := NewChatSession("user_123", []string{"match_abc"}, "q?", "a.", "clickhouse", nil, 7)
	if err != nil {
		t.Fatalf("first session creation failed: %v", err)
	}
	s2, err := NewChatSession("user_123", []string{"match_abc"}, "q?", "a.", "clickhouse", nil, 7)
	if err != nil {
		t.Fatalf("second session creation failed: %v", err)
	}
	if s1.ID == s2.ID {
		t.Error("ULID should be unique between calls, but got identical IDs")
	}
}

func TestChatSession_Valid(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name    string
		session ChatSession
		wantErr error
	}{
		{
			name: "valid session",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: []Message{
					{
						Question:  "What happened?",
						Answer:    "Something happened.",
						Source:    "clickhouse",
						CreatedAt: time.Now(),
					},
				},
				ExpiresAt: future,
			},
			wantErr: nil,
		},
		{
			name: "empty user ID",
			session: ChatSession{
				UserID:   "",
				MatchIDs: []string{"match_abc"},
				Messages: []Message{
					{
						Question:  "What happened?",
						Answer:    "Something happened.",
						Source:    "clickhouse",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: ErrSessionUserIDRequired,
		},
		{
			name: "no match IDs",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{},
				Messages: []Message{
					{
						Question:  "What happened?",
						Answer:    "Something happened.",
						Source:    "clickhouse",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: ErrSessionNoMatches,
		},
		{
			name: "no messages",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: nil,
			},
			wantErr: ErrSessionQuestionRequired,
		},
		{
			name: "empty messages slice",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: []Message{},
			},
			wantErr: ErrSessionQuestionRequired,
		},
		{
			name: "message with empty question",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: []Message{
					{
						Question:  "",
						Answer:    "Something happened.",
						Source:    "clickhouse",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: ErrSessionQuestionRequired,
		},
		{
			name: "message with question too long",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: []Message{
					{
						Question:  strings.Repeat("a", 501),
						Answer:    "Something happened.",
						Source:    "clickhouse",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: ErrSessionQuestionTooLong,
		},
		{
			name: "message with empty answer",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: []Message{
					{
						Question:  "What happened?",
						Answer:    "",
						Source:    "clickhouse",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: ErrSessionAnswerRequired,
		},
		{
			name: "message with invalid source",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: []Message{
					{
						Question:  "What happened?",
						Answer:    "Something happened.",
						Source:    "invalid",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: ErrInvalidSource,
		},
		{
			name: "message with qdrant source",
			session: ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: []Message{
					{
						Question:  "What happened?",
						Answer:    "Something happened.",
						Source:    "qdrant",
						CreatedAt: time.Now(),
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Valid()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddMessage(t *testing.T) {
	tests := []struct {
		name       string
		setupFn    func() *ChatSession
		question   string
		answer     string
		source     string
		dataPoints []DataPoint
		wantErr    error
	}{
		{
			name: "successfully add message",
			setupFn: func() *ChatSession {
				s, _ := NewChatSession("user_123", []string{"match_abc"}, "First question?", "First answer.", "clickhouse", nil, 7)
				return s
			},
			question:   "Second question?",
			answer:     "Second answer.",
			source:     "clickhouse",
			dataPoints: []DataPoint{{Label: "kills", Value: "10"}},
			wantErr:    nil,
		},
		{
			name: "add with empty question",
			setupFn: func() *ChatSession {
				s, _ := NewChatSession("user_123", []string{"match_abc"}, "First question?", "First answer.", "clickhouse", nil, 7)
				return s
			},
			question: "",
			answer:   "Second answer.",
			source:   "clickhouse",
			wantErr:  ErrSessionQuestionRequired,
		},
		{
			name: "add with empty answer",
			setupFn: func() *ChatSession {
				s, _ := NewChatSession("user_123", []string{"match_abc"}, "First question?", "First answer.", "clickhouse", nil, 7)
				return s
			},
			question: "Second question?",
			answer:   "",
			source:   "clickhouse",
			wantErr:  ErrSessionAnswerRequired,
		},
		{
			name: "add with invalid source",
			setupFn: func() *ChatSession {
				s, _ := NewChatSession("user_123", []string{"match_abc"}, "First question?", "First answer.", "clickhouse", nil, 7)
				return s
			},
			question: "Second question?",
			answer:   "Second answer.",
			source:   "invalid",
			wantErr:  ErrInvalidSource,
		},
		{
			name: "max messages reached",
			setupFn: func() *ChatSession {
				s, _ := NewChatSession("user_123", []string{"match_abc"}, "First question?", "First answer.", "clickhouse", nil, 7)
				// Add 49 more messages to reach exactly 50
				for i := 0; i < 49; i++ {
					_ = s.AddMessage("q?", "a.", "clickhouse", nil)
				}
				return s
			},
			question: "One more?",
			answer:   "No more.",
			source:   "clickhouse",
			wantErr:  ErrMaxMessagesReached,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setupFn()
			beforeUpdate := s.UpdatedAt
			beforeCount := len(s.Messages)

			// Small sleep to ensure time difference is measurable
			time.Sleep(time.Millisecond)

			err := s.AddMessage(tt.question, tt.answer, tt.source, tt.dataPoints)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("AddMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == nil {
				if len(s.Messages) != beforeCount+1 {
					t.Errorf("expected %d messages, got %d", beforeCount+1, len(s.Messages))
				}
				if !s.UpdatedAt.After(beforeUpdate) {
					t.Error("UpdatedAt should have been updated")
				}
				lastMsg := s.Messages[len(s.Messages)-1]
				if lastMsg.Question != tt.question {
					t.Errorf("unexpected question: got %q, want %q", lastMsg.Question, tt.question)
				}
				if lastMsg.Answer != tt.answer {
					t.Errorf("unexpected answer: got %q, want %q", lastMsg.Answer, tt.answer)
				}
				if lastMsg.Source != tt.source {
					t.Errorf("unexpected source: got %q, want %q", lastMsg.Source, tt.source)
				}
			} else if errors.Is(err, ErrMaxMessagesReached) {
				if len(s.Messages) != beforeCount {
					t.Errorf("message count should not change on error: %d", len(s.Messages))
				}
			}
		})
	}
}

func TestAddMessage_UpdatesUpdatedAt(t *testing.T) {
	s, err := NewChatSession("user_123", []string{"match_abc"}, "First?", "First answer.", "clickhouse", nil, 7)
	if err != nil {
		t.Fatalf("NewChatSession() unexpected error: %v", err)
	}

	originalUpdatedAt := s.UpdatedAt

	time.Sleep(time.Millisecond)

	if err := s.AddMessage("Second?", "Second answer.", "clickhouse", nil); err != nil {
		t.Fatalf("AddMessage() unexpected error: %v", err)
	}

	if !s.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be after the original UpdatedAt")
	}

	if len(s.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(s.Messages))
	}
}

func TestAddMessage_ExactLimit(t *testing.T) {
	s, err := NewChatSession("user_123", []string{"match_abc"}, "First?", "First.", "clickhouse", nil, 7)
	if err != nil {
		t.Fatalf("NewChatSession() unexpected error: %v", err)
	}

	// Add 49 more messages to reach exactly 50 (1 initial + 49 = 50)
	for i := 0; i < 49; i++ {
		if err := s.AddMessage("q?", "a.", "clickhouse", nil); err != nil {
			t.Fatalf("AddMessage() iteration %d unexpected error: %v", i, err)
		}
	}

	if len(s.Messages) != MaxMessagesPerSession {
		t.Fatalf("expected %d messages, got %d", MaxMessagesPerSession, len(s.Messages))
	}

	// The 51st attempt (index 50) should fail
	err = s.AddMessage("One more?", "No more.", "clickhouse", nil)
	if !errors.Is(err, ErrMaxMessagesReached) {
		t.Errorf("expected ErrMaxMessagesReached, got %v", err)
	}

	if len(s.Messages) != MaxMessagesPerSession {
		t.Errorf("message count should remain %d after limit error, got %d", MaxMessagesPerSession, len(s.Messages))
	}
}

func TestLastNMessages(t *testing.T) {
	// Build a session with 15 messages
	createMessages := func(n int) []Message {
		msgs := make([]Message, n)
		for i := 0; i < n; i++ {
			msgs[i] = Message{
				Question:  "q" + string(rune('0'+i)),
				Answer:    "a" + string(rune('0'+i)),
				Source:    "clickhouse",
				CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
			}
		}
		return msgs
	}

	tests := []struct {
		name     string
		messages []Message
		n        int
		wantLen  int
		wantNil  bool
	}{
		{
			name:     "zero returns nil",
			messages: createMessages(5),
			n:        0,
			wantNil:  true,
		},
		{
			name:     "negative returns nil",
			messages: createMessages(5),
			n:        -1,
			wantNil:  true,
		},
		{
			name:     "n larger than messages returns all",
			messages: createMessages(5),
			n:        10,
			wantLen:  5,
		},
		{
			name:     "n equals exact count returns all",
			messages: createMessages(5),
			n:        5,
			wantLen:  5,
		},
		{
			name:     "partial returns last n",
			messages: createMessages(15),
			n:        10,
			wantLen:  10,
		},
		{
			name:     "n=1 returns last message",
			messages: createMessages(10),
			n:        1,
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ChatSession{
				UserID:   "user_123",
				MatchIDs: []string{"match_abc"},
				Messages: tt.messages,
			}

			got := s.LastNMessages(tt.n)

			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}

			if len(got) != tt.wantLen {
				t.Errorf("expected %d messages, got %d", tt.wantLen, len(got))
			}

			// Verify chronological order (oldest first among returned)
			if len(got) >= 2 {
				if got[0].CreatedAt.After(got[1].CreatedAt) {
					t.Error("messages not in chronological order (oldest first)")
				}
			}

			// For partial case, verify we got the last n messages
			if tt.n < len(tt.messages) && tt.n > 0 {
				expectedStartIdx := len(tt.messages) - tt.n
				for i := 0; i < tt.n; i++ {
					if got[i].Question != tt.messages[expectedStartIdx+i].Question {
						t.Errorf("message %d mismatch at index %d: got %q, want %q",
							i, expectedStartIdx+i, got[i].Question, tt.messages[expectedStartIdx+i].Question)
					}
				}
			}
		})
	}
}

func TestLastNMessages_OldestFirstOrder(t *testing.T) {
	// Explicitly test chronological ordering
	messages := []Message{
		{Question: "first", Answer: "a", Source: "clickhouse", CreatedAt: time.Now().Add(-10 * time.Minute)},
		{Question: "second", Answer: "a", Source: "clickhouse", CreatedAt: time.Now().Add(-5 * time.Minute)},
		{Question: "third", Answer: "a", Source: "clickhouse", CreatedAt: time.Now()},
	}

	s := &ChatSession{
		UserID:   "user_123",
		MatchIDs: []string{"match_abc"},
		Messages: messages,
	}

	got := s.LastNMessages(2)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}

	if got[0].Question != "second" || got[1].Question != "third" {
		t.Errorf("expected [second, third] in order, got [%s, %s]", got[0].Question, got[1].Question)
	}

	// Verify the returned order is chronological (oldest first)
	if got[0].CreatedAt.After(got[1].CreatedAt) {
		t.Error("returned messages not in chronological order")
	}
}

func TestLastNMessages_Immutability(t *testing.T) {
	messages := []Message{
		{Question: "first", Answer: "a", Source: "clickhouse"},
		{Question: "second", Answer: "a", Source: "clickhouse"},
	}

	s := &ChatSession{
		UserID:   "user_123",
		MatchIDs: []string{"match_abc"},
		Messages: messages,
	}

	// When n >= len(Messages), should return a copy
	got := s.LastNMessages(10)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}

	// Modifying the returned slice should not affect the original
	got[0].Question = "modified"
	if s.Messages[0].Question != "first" {
		t.Error("modifying returned slice should not affect original session messages")
	}
}

func TestConstants(t *testing.T) {
	if MaxQuestionLen != 500 {
		t.Errorf("MaxQuestionLen = %d, want 500", MaxQuestionLen)
	}
	if MaxMessagesPerSession != 50 {
		t.Errorf("MaxMessagesPerSession = %d, want 50", MaxMessagesPerSession)
	}
	if MaxMessagesInContext != 10 {
		t.Errorf("MaxMessagesInContext = %d, want 10", MaxMessagesInContext)
	}
}

func TestErrorSentinels(t *testing.T) {
	// Verify all sentinel errors are exported and non-nil
	tests := []struct {
		name string
		err  error
	}{
		{"ErrSessionUserIDRequired", ErrSessionUserIDRequired},
		{"ErrSessionNoMatches", ErrSessionNoMatches},
		{"ErrSessionQuestionRequired", ErrSessionQuestionRequired},
		{"ErrSessionQuestionTooLong", ErrSessionQuestionTooLong},
		{"ErrSessionAnswerRequired", ErrSessionAnswerRequired},
		{"ErrInvalidSource", ErrInvalidSource},
		{"ErrMaxMessagesReached", ErrMaxMessagesReached},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("sentinel error should not be nil")
			}
			if tt.err.Error() == "" {
				t.Error("sentinel error message should not be empty")
			}
		})
	}
}

func BenchmarkChatSession_Valid(b *testing.B) {
	s, err := NewChatSession(
		"user_123",
		[]string{"match_abc", "match_def"},
		"What happened?",
		"Something happened.",
		"clickhouse",
		[]DataPoint{{Label: "player", Value: "FalleN"}},
		7,
	)
	if err != nil {
		b.Fatalf("NewChatSession() error: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Valid()
	}
}

func BenchmarkChatSession_LastNMessages(b *testing.B) {
	messages := make([]Message, 50)
	for i := 0; i < 50; i++ {
		messages[i] = Message{
			Question:  "q" + string(rune('0'+i)),
			Answer:    "a" + string(rune('0'+i)),
			Source:    "clickhouse",
			CreatedAt: time.Now(),
		}
	}
	s := &ChatSession{
		UserID:   "user_123",
		MatchIDs: []string{"match_abc"},
		Messages: messages,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.LastNMessages(10)
	}
}
