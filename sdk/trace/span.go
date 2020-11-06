// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace // import "go.opentelemetry.io/otel/sdk/trace"

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/global"
	"go.opentelemetry.io/otel/label"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/internal"
)

const (
	errorTypeKey    = label.Key("error.type")
	errorMessageKey = label.Key("error.message")
	errorEventName  = "error"
)

var emptySpanContext = otel.SpanContext{}

// span is an implementation of the OpenTelemetry Span API representing the
// individual component of a trace.
type span struct {
	// data contains information recorded about the span.
	//
	// It will be non-nil if we are exporting the span or recording events for it.
	// Otherwise, data is nil, and the span is simply a carrier for the
	// SpanContext, so that the trace ID is propagated.
	data        *export.SpanData
	mu          sync.Mutex // protects the contents of *data (but not the pointer value.)
	spanContext otel.SpanContext

	// attributes are capped at configured limit. When the capacity is reached an oldest entry
	// is removed to create room for a new entry.
	attributes *attributesMap

	// messageEvents are stored in FIFO queue capped by configured limit.
	messageEvents *evictedQueue

	// links are stored in FIFO queue capped by configured limit.
	links *evictedQueue

	// spanStore is the spanStore this span belongs to, if any, otherwise it is nil.
	//*spanStore
	endOnce sync.Once

	// done is the completion state of the span. Use the ended method to check
	// this in a concurrently safe manner.
	done bool
	// doneMu protects the done field.
	doneMu sync.RWMutex

	executionTracerTaskEnd func()  // ends the execution tracer span
	tracer                 *tracer // tracer used to create span.
}

var _ otel.Span = &span{}

// ended returns if s has ended and should not be updated.
func (s *span) ended() bool {
	s.doneMu.RLock()
	defer s.doneMu.RUnlock()
	return s.done
}

// ignoreUpdates returns if updates to the span should be ignored.
func (s *span) ignoreUpdates() bool {
	return !s.IsRecording() || s.ended()
}

// SpanContext returns the SpanContext of s. The returned SpanContext is
// usable even after End has been called for s.
func (s *span) SpanContext() otel.SpanContext {
	if s == nil {
		return otel.SpanContext{}
	}
	return s.spanContext
}

// IsRecording returns the recording state of s. It will return true if s is
// active and events can be recorded.
func (s *span) IsRecording() bool {
	return s != nil && s.data != nil
}

// SetStatus sets the status of s with code and msg. Any previous status s had
// will be overwritten. If s is not recording or is ended the status of s will
// not be updated and this method will perform no operation.
func (s *span) SetStatus(code codes.Code, msg string) {
	if s.ignoreUpdates() {
		return
	}
	s.mu.Lock()
	s.data.StatusCode = code
	s.data.StatusMessage = msg
	s.mu.Unlock()
}

// SetAttributes sets attributes as attributes of s. If a key from attributes
// already exists for an attribute of s it will be overwritten with the value
// contained in the passed attributes. If s is not recording or is ended the
// attributes of s will not be updated and this method will perform no
// operation.
func (s *span) SetAttributes(attributes ...label.KeyValue) {
	if s.ignoreUpdates() {
		return
	}
	s.copyToCappedAttributes(attributes...)
}

// End ends the span.
//
// The only SpanOption currently supported is WithTimestamp which will set the
// end time for a Span's life-cycle.
//
// If this method is called while panicking an error event is added to the
// Span before ending it and the panic is continued.
func (s *span) End(options ...otel.SpanOption) {
	if s == nil {
		return
	}

	recovered := recover()
	if recovered != nil {
		// Record but do not stop the panic.
		defer panic(recovered)
	}

	s.endOnce.Do(func() {
		// Ensure s is considered ended and calls to update s are ignored
		// before the SpanData is finalized.
		s.doneMu.Lock()
		s.done = true
		s.doneMu.Unlock()

		// End the execution tracer regardless of s recording or not.
		if s.executionTracerTaskEnd != nil {
			s.executionTracerTaskEnd()
		}

		// If s is not recording no need to add events or process.
		if !s.IsRecording() {
			return
		}

		// Ensure there is a processing destination before we finalize.
		sps, ok := s.tracer.provider.spanProcessors.Load().(spanProcessorStates)
		if !ok || len(sps) < 1 {
			return
		}

		if recovered != nil {
			s.addEvent(errorEventName, otel.WithAttributes(
				errorTypeKey.String(typeStr(recovered)),
				errorMessageKey.String(fmt.Sprint(recovered)),
			))
		}

		sd := s.finalSpanData(options)
		for _, sp := range sps {
			sp.sp.OnEnd(sd)
		}
	})
}

// RecordError records err as a Span event with the provided options. If s is
// not recording or is ended this method performs no operation and no event is
// added.
func (s *span) RecordError(err error, opts ...otel.EventOption) {
	if err == nil || s.ignoreUpdates() {
		return
	}

	s.SetStatus(codes.Error, "")
	opts = append(opts, otel.WithAttributes(
		errorTypeKey.String(typeStr(err)),
		errorMessageKey.String(err.Error()),
	))
	s.addEvent(errorEventName, opts...)
}

func typeStr(i interface{}) string {
	t := reflect.TypeOf(i)
	if t.PkgPath() == "" && t.Name() == "" {
		// Likely a builtin type.
		return t.String()
	}
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

// Tracer returns the Tracer that created s.
func (s *span) Tracer() otel.Tracer {
	return s.tracer
}

// AddEvent adds an event with the provided name and options. If s is not
// recording or is ended this method performs no operation and no event is
// added.
func (s *span) AddEvent(name string, o ...otel.EventOption) {
	if s.ignoreUpdates() {
		return
	}
	s.addEvent(name, o...)
}

func (s *span) addEvent(name string, o ...otel.EventOption) {
	c := otel.NewEventConfig(o...)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageEvents.add(export.Event{
		Name:       name,
		Attributes: c.Attributes,
		Time:       c.Timestamp,
	})
}

var errUninitializedSpan = errors.New("failed to set name on uninitialized span")

// SetName sets the name of s. If s is not recording or is ended this method
// performs no operation and the name of s will not be updated.
func (s *span) SetName(name string) {
	if s == nil || s.ended() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data == nil {
		global.Handle(errUninitializedSpan)
		return
	}
	s.data.Name = name
	// SAMPLING
	noParent := !s.data.ParentSpanID.IsValid()
	var ctx otel.SpanContext
	if noParent {
		ctx = otel.SpanContext{}
	} else {
		// FIXME: Where do we get the parent context from?
		// From SpanStore?
		ctx = s.data.SpanContext
	}
	data := samplingData{
		noParent:     noParent,
		remoteParent: s.data.HasRemoteParent,
		parent:       ctx,
		name:         name,
		cfg:          s.tracer.provider.config.Load().(*Config),
		span:         s,
		attributes:   s.data.Attributes,
		links:        s.data.Links,
		kind:         s.data.SpanKind,
	}
	sampled := makeSamplingDecision(data)

	// Adding attributes directly rather than using s.SetAttributes()
	// as s.mu is already locked and attempting to do so would deadlock.
	for _, a := range sampled.Attributes {
		s.attributes.add(a)
	}
}

// addLink adds a span link with to s. If s is not recording or is ended this
// method performs no operation and no link is added.
func (s *span) addLink(link otel.Link) {
	if s.ignoreUpdates() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.links.add(link)
}

// finalSpanData produces SpanData representing the finalized state of the
// span. It requires that s.data is non-nil.
func (s *span) finalSpanData(opts []otel.SpanOption) *export.SpanData {
	var sd export.SpanData
	s.mu.Lock()
	defer s.mu.Unlock()
	sd = *s.data

	s.attributes.toSpanData(&sd)

	if len(s.messageEvents.queue) > 0 {
		sd.MessageEvents = s.interfaceArrayToMessageEventArray()
		sd.DroppedMessageEventCount = s.messageEvents.droppedCount
	}
	if len(s.links.queue) > 0 {
		sd.Links = s.interfaceArrayToLinksArray()
		sd.DroppedLinkCount = s.links.droppedCount
	}

	config := otel.NewSpanConfig(opts...)
	if config.Timestamp.IsZero() {
		sd.EndTime = internal.MonotonicEndTime(sd.StartTime)
	} else {
		sd.EndTime = config.Timestamp
	}

	return &sd
}

func (s *span) interfaceArrayToLinksArray() []otel.Link {
	linkArr := make([]otel.Link, 0)
	for _, value := range s.links.queue {
		linkArr = append(linkArr, value.(otel.Link))
	}
	return linkArr
}

func (s *span) interfaceArrayToMessageEventArray() []export.Event {
	messageEventArr := make([]export.Event, 0)
	for _, value := range s.messageEvents.queue {
		messageEventArr = append(messageEventArr, value.(export.Event))
	}
	return messageEventArr
}

func (s *span) copyToCappedAttributes(attributes ...label.KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range attributes {
		if a.Value.Type() != label.INVALID {
			s.attributes.add(a)
		}
	}
}

// addChild increments the child span count of s. If s is not recording or is
// ended this method performs no operation and no additional child spans will
// be counted.
func (s *span) addChild() {
	if s.ignoreUpdates() {
		return
	}
	s.mu.Lock()
	s.data.ChildSpanCount++
	s.mu.Unlock()
}

func startSpanInternal(tr *tracer, name string, parent otel.SpanContext, remoteParent bool, o *otel.SpanConfig) *span {
	var noParent bool
	span := &span{}
	span.spanContext = parent

	cfg := tr.provider.config.Load().(*Config)

	if parent == emptySpanContext {
		span.spanContext.TraceID = cfg.IDGenerator.NewTraceID()
		noParent = true
	}
	span.spanContext.SpanID = cfg.IDGenerator.NewSpanID()
	data := samplingData{
		noParent:     noParent,
		remoteParent: remoteParent,
		parent:       parent,
		name:         name,
		cfg:          cfg,
		span:         span,
		attributes:   o.Attributes,
		links:        o.Links,
		kind:         o.SpanKind,
	}
	sampled := makeSamplingDecision(data)

	// TODO: [rghetia] restore when spanstore is added.
	// if !internal.LocalSpanStoreEnabled && !span.spanContext.IsSampled() && !o.Record {
	if !span.spanContext.IsSampled() && !o.Record {
		return span
	}

	startTime := o.Timestamp
	if startTime.IsZero() {
		startTime = time.Now()
	}
	span.data = &export.SpanData{
		SpanContext:            span.spanContext,
		StartTime:              startTime,
		SpanKind:               otel.ValidateSpanKind(o.SpanKind),
		Name:                   name,
		HasRemoteParent:        remoteParent,
		Resource:               cfg.Resource,
		InstrumentationLibrary: tr.instrumentationLibrary,
	}
	span.attributes = newAttributesMap(cfg.MaxAttributesPerSpan)
	span.messageEvents = newEvictedQueue(cfg.MaxEventsPerSpan)
	span.links = newEvictedQueue(cfg.MaxLinksPerSpan)

	span.SetAttributes(sampled.Attributes...)

	if !noParent {
		span.data.ParentSpanID = parent.SpanID
	}
	// TODO: [rghetia] restore when spanstore is added.
	//if internal.LocalSpanStoreEnabled {
	//	ss := spanStoreForNameCreateIfNew(name)
	//	if ss != nil {
	//		span.spanStore = ss
	//		ss.add(span)
	//	}
	//}

	return span
}

type samplingData struct {
	noParent     bool
	remoteParent bool
	parent       otel.SpanContext
	name         string
	cfg          *Config
	span         *span
	attributes   []label.KeyValue
	links        []otel.Link
	kind         otel.SpanKind
}

func makeSamplingDecision(data samplingData) SamplingResult {
	sampler := data.cfg.DefaultSampler
	spanContext := &data.span.spanContext
	sampled := sampler.ShouldSample(SamplingParameters{
		ParentContext:   data.parent,
		TraceID:         spanContext.TraceID,
		Name:            data.name,
		HasRemoteParent: data.remoteParent,
		Kind:            data.kind,
		Attributes:      data.attributes,
		Links:           data.links,
	})
	if sampled.Decision == RecordAndSample {
		spanContext.TraceFlags |= otel.FlagsSampled
	} else {
		spanContext.TraceFlags &^= otel.FlagsSampled
	}
	return sampled
}
