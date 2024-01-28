package sse_test

import (
	"testing"
	"time"

	"github.com/tmaxmax/go-sse"
	"github.com/tmaxmax/go-sse/internal/tests"
)

func replay(tb testing.TB, p sse.ReplayProvider, lastEventID sse.EventID, topics ...string) []*sse.Message {
	tb.Helper()

	if len(topics) == 0 {
		topics = []string{sse.DefaultTopic}
	}

	var replayed []*sse.Message
	cb := mockClient(func(m *sse.Message) error {
		if m != nil {
			replayed = append(replayed, m)
		}
		return nil
	})

	sub := sse.Subscription{
		Client:      cb,
		LastEventID: lastEventID,
		Topics:      topics,
	}

	_ = p.Replay(sub)

	sub.LastEventID = sse.EventID{}
	_ = p.Replay(sub)

	sub.LastEventID = sse.ID("mama")
	_ = p.Replay(sub)

	sub.LastEventID = sse.ID("10")
	_ = p.Replay(sub)

	return replayed
}

func testReplayError(tb testing.TB, p sse.ReplayProvider, tm *tests.Time) {
	tb.Helper()

	tm.Reset()
	tm.Add(time.Hour)

	p.Put(msg(tb, "a", "1"), []string{sse.DefaultTopic})
	p.Put(msg(tb, "b", "2"), []string{sse.DefaultTopic})

	cb := mockClient(func(_ *sse.Message) error { return nil })

	tm.Rewind()

	err := p.Replay(sse.Subscription{
		Client:      cb,
		LastEventID: sse.ID("1"),
		Topics:      []string{sse.DefaultTopic},
	})

	tests.Equal(tb, err, nil, "received invalid error")
}

func TestValidReplayProvider(t *testing.T) {
	t.Parallel()

	tm := &tests.Time{}
	p := &sse.ValidReplayProvider{
		TTL:     time.Millisecond * 5,
		AutoIDs: true,
		Now:     tm.Now,
	}

	tests.Equal(t, p.GC(), nil, "unexpected GC error") // no elements, noop
	tests.Equal(t, p.Replay(sse.Subscription{}), nil, "replay failed on provider without messages")

	now := time.Now()
	initialNow := now
	tm.Set(now)

	p.Put(msg(t, "hi", ""), []string{sse.DefaultTopic})
	p.Put(msg(t, "there", ""), []string{"t"})
	tm.Add(p.TTL)
	p.Put(msg(t, "world", ""), []string{sse.DefaultTopic})
	p.Put(msg(t, "again", ""), []string{"t"})
	tm.Add(p.TTL * 3)
	p.Put(msg(t, "world", ""), []string{sse.DefaultTopic})
	p.Put(msg(t, "x", ""), []string{"t"})
	tm.Add(p.TTL * 5)
	p.Put(msg(t, "again", ""), []string{"t"})

	tm.Set(initialNow.Add(p.TTL))

	tests.Equal(t, p.GC(), nil, "unexpected GC error")

	tm.Set(now.Add(p.TTL))

	replayed := replay(t, p, sse.ID("3"), sse.DefaultTopic, "topic with no messages")[0]
	tests.Equal(t, replayed.String(), "id: 4\ndata: world\n\n", "invalid message received")

	testReplayError(t, &sse.ValidReplayProvider{Now: tm.Now}, tm)
}

func TestFiniteReplayProvider(t *testing.T) {
	t.Parallel()

	p := &sse.FiniteReplayProvider{Count: 3}

	tests.Equal(t, p.Replay(sse.Subscription{}), nil, "replay failed on provider without messages")

	r := tests.Panics(t, func() {
		p.Put(&sse.Message{Type: sse.Type("panic")}, []string{sse.DefaultTopic})
	}, "messages without IDs cannot be put in a replay provider")
	rerr, isErr := r.(error)
	tests.Expect(t, isErr, "should panic with error")
	tests.Equal(t, rerr.Error(), `go-sse: a Message without an ID was given to a provider that doesn't set IDs automatically.
The message is the following:
│ event: panic
└─■`, "invalid panic error message")

	r = tests.Panics(t, func() {
		p.Put(&sse.Message{ID: sse.ID("5"), Type: sse.Type("panic")}, nil)
	}, "messages cannot be put without a topic")
	rerr, isErr = r.(error)
	tests.Expect(t, isErr, "should panic with error")
	tests.Equal(t, rerr.Error(), `go-sse: no topics provided for Message.
The message is the following:
│ id: 5
│ event: panic
└─■`, "invalid panic error message")

	p.Put(msg(t, "", "1"), []string{sse.DefaultTopic})
	p.Put(msg(t, "hello", "2"), []string{sse.DefaultTopic})
	p.Put(msg(t, "there", "3"), []string{"t"})
	p.Put(msg(t, "world", "4"), []string{sse.DefaultTopic})

	replayed := replay(t, p, sse.ID("2"))[0]
	tests.Equal(t, replayed.String(), "id: 4\ndata: world\n\n", "invalid replayed message")

	p.Put(msg(t, "", "5"), []string{"t"})
	p.Put(msg(t, "", "6"), []string{"t"})
	p.Put(msg(t, "again", "7"), []string{sse.DefaultTopic})

	replayed = replay(t, p, sse.ID("4"), sse.DefaultTopic, "topic with no messages")[0]
	tests.Equal(t, replayed.String(), "id: 7\ndata: again\n\n", "invalid replayed message")

	testReplayError(t, &sse.FiniteReplayProvider{Count: 10}, nil)
}
