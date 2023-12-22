package runner

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	ulid "github.com/stateful/runme/internal/ulid"
	"go.uber.org/zap"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var tp *sdktrace.TracerProvider

// initTracer creates and registers trace provider instance.
func init() {
	traceProvider()
}

func traceProvider() {
	// stdr.SetVerbosity(5)

	ppExp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal("failed to initialize stdouttrace exporter", err)
	}
	ppBsp := sdktrace.NewBatchSpanProcessor(ppExp)
	ctx := context.Background()

	grpcExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		log.Fatal("failed to initialize otlptracegrpc exporter", err)
	}
	grpcBsp := sdktrace.NewBatchSpanProcessor(grpcExp)
	tp = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(grpcBsp),
		sdktrace.WithSpanProcessor(ppBsp),
	)
	otel.SetTracerProvider(tp)
}

// Session is an abstract entity separate from
// an execution. Currently, its main role is to
// keep track of environment variables.
type Session struct {
	ID       string
	Metadata map[string]string

	envStore *envStore
	logger   *zap.Logger
	Tracer   trace.Tracer
	Span     trace.Span
	Context  context.Context
}

func NewSession(envs []string, logger *zap.Logger) (*Session, error) {
	sessionEnvs := []string(envs)
	ID := ulid.GenerateID()
	tracer := tp.Tracer("session")

	fileName := "session.md"
	m0, _ := baggage.NewMember(string("SessionID"), ID)
	m1, _ := baggage.NewMember(string("filename"), fileName)
	b, _ := baggage.New(m0, m1)
	ctx := context.Background()
	ctx = baggage.ContextWithBaggage(ctx, b)
	ctx, span := tracer.Start(ctx, "Session")

	sidKey := attribute.Key("SessionID")
	fnKey := attribute.Key("Filename")

	span.SetAttributes(sidKey.String(ID), fnKey.String(fileName))

	s := &Session{
		ID: ID,

		envStore: newEnvStore(sessionEnvs...),
		logger:   logger,
		Tracer:   tracer,
		Span:     span,
		Context:  ctx,
	}
	return s, nil
}

func (s *Session) EndSpan() {
	s.Span.End(trace.WithTimestamp(time.Now()))
}

func (s *Session) AddEnvs(envs []string) {
	s.envStore.Add(envs...)
}

func (s *Session) Envs() []string {
	return s.envStore.Values()
}

// thread-safe session list
type SessionList struct {
	// WARNING: this mutex is created to prevent race conditions on certain
	// operations, like finding the most recent element of store, which are not
	// supported by golang-lru
	//
	// this is not really ideal since this introduces a chance of deadlocks.
	// please make sure that this mutex is never locked within the critical
	// section of the inner lock (belonging to store)
	mu    sync.RWMutex
	store *lru.Cache[string, *Session]
}

func NewSessionList() (*SessionList, error) {
	// max integer
	capacity := int(^uint(0) >> 1)

	store, err := lru.New[string, *Session](capacity)
	if err != nil {
		return nil, err
	}

	return &SessionList{
		store: store,
	}, nil
}

func (sl *SessionList) AddSession(session *Session) {
	sl.store.Add(session.ID, session)
}

func (sl *SessionList) CreateAndAddSession(generate func() (*Session, error)) (*Session, error) {
	sess, err := generate()
	if err != nil {
		return nil, err
	}

	sl.AddSession(sess)
	return sess, nil
}

func (sl *SessionList) GetSession(id string) (*Session, bool) {
	return sl.store.Get(id)
}

func (sl *SessionList) DeleteSession(id string) (present bool) {
	s, ok := sl.GetSession(id)
	if ok {
		s.EndSpan()
	}

	return sl.store.Remove(id)
}

func (sl *SessionList) MostRecent() (*Session, bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	return sl.mostRecentUnsafe()
}

func (sl *SessionList) mostRecentUnsafe() (*Session, bool) {
	keys := sl.store.Keys()

	sl.store.GetOldest()

	if len(keys) == 0 {
		return nil, false
	}

	return sl.store.Peek(keys[len(keys)-1])
}

func (sl *SessionList) MostRecentOrCreate(generate func() (*Session, error)) (*Session, error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if existing, ok := sl.mostRecentUnsafe(); ok {
		return existing, nil
	}

	return sl.CreateAndAddSession(generate)
}

func (sl *SessionList) ListSessions() ([]*Session, error) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	keys := sl.store.Keys()
	sessions := make([]*Session, len(keys))

	for i, k := range keys {
		sess, ok := sl.store.Peek(k)
		if !ok {
			return nil, fmt.Errorf("unexpected error: unable to find session %s when listing sessions from lru cache", sess.ID)
		}
		sessions[i] = sess
	}

	return sessions, nil
}
