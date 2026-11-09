// Package editor exposes reusable Zed editor instances over protobuf and Sable.
// The native backend links GPL-3.0-or-later components. The Go API and custom
// transports can be used without linking a native backend.
package editor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/editor/v1"
	"google.golang.org/protobuf/proto"
)

type Operation uint32

const (
	OperationCommand Operation = 0x5a454401
	OperationEvents  Operation = 0x5a454402
	MaxTextBytes               = 1 << 20
	maxMessageBytes            = MaxTextBytes + 8192
)

// Transport must return independently owned response bytes and respect context
// cancellation. It must not retain request bytes without copying them.
type Transport interface {
	Call(context.Context, Operation, []byte) ([]byte, error)
}

type Code int32

const (
	InvalidArgument    Code = 1
	NotFound           Code = 2
	ReadOnly           Code = 3
	ResourceExhausted  Code = 4
	Closed             Code = 5
	Internal           Code = 6
	UnsupportedVersion Code = 7
)

var (
	ErrClosed            = errors.New("editor: host closed")
	ErrReadOnly          = errors.New("editor: read-only")
	ErrNotFound          = errors.New("editor: closed or unknown handle")
	ErrConcurrentEvents  = errors.New("editor: another event consumer is active")
	ErrNativeUnavailable = errors.New("editor: build with gpui_zed_editor,sable_extern_lib,sable_portable and link libgpui_go_zed_editor_host (see make editor)")
)

type ProtocolError struct {
	Code    Code
	Message string
}

func (e *ProtocolError) Error() string { return fmt.Sprintf("editor: code %d: %s", e.Code, e.Message) }
func (e *ProtocolError) Is(target error) bool {
	return (e.Code == Closed && target == ErrClosed) || (e.Code == ReadOnly && target == ErrReadOnly) || (e.Code == NotFound && target == ErrNotFound)
}
func protocolError(value *pb.Error) *ProtocolError {
	return &ProtocolError{Code: Code(value.Code), Message: value.Message}
}

type Capabilities struct{ Major, MaxTextBytes, MaxEditors uint32 }

// Options configure a new local editor window in the supplied standalone host.
// Zero dimensions select 1000x700. Gutter and soft wrap are enabled by default.
type Options struct {
	Title           string
	Width, Height   float32
	Text            string
	ReadOnly        bool
	HideGutter      bool
	DisableSoftWrap bool
	// WidthTable chooses how a Position counts East Asian Ambiguous characters.
	// It does not change how they are drawn.
	WidthTable WidthTable
}

// Position is a zero-based location. Line is a buffer line, and Column counts
// visual cells from its start, as the editor lays them out and by the East
// Asian Width table: a wide or fullwidth character takes two, a tab reaches its
// stop, and a combining mark takes none. An ambiguous character follows the
// editor's WidthTable. Soft wrapping does not restart the count. Column does not
// index a Go string.
//
// Character is an alternative to Column when setting a selection: the number of
// Unicode code points from the start of the line, as counted by
// utf8.RuneCountInString. Setting both nonzero returns InvalidArgument.
//
// Character and Clamp apply only when setting a selection. Reported positions
// leave them zero.
type Position struct {
	Line, Column, Character uint32
	Clamp                   Clamp
}

// WidthTable chooses how positions count East Asian Ambiguous characters. The
// editor draws glyphs from the font either way, so choose the table that matches
// the font: Japanese fonts usually draw these characters two columns wide.
type WidthTable int32

const (
	// WidthNarrowAmbiguous counts every ambiguous character as one column.
	WidthNarrowAmbiguous WidthTable = iota
	// WidthWideAmbiguous counts every ambiguous character that has a width as
	// two columns, including letters such as α and é.
	WidthWideAmbiguous
	// WidthWideSymbols counts ambiguous symbols, punctuation and numbers as two
	// columns and ambiguous letters as one.
	WidthWideSymbols
)

// Clamp says where a Position inside a wide character, a tab or a grapheme
// cluster goes.
type Clamp int32

const (
	// ClampNone rejects the position with InvalidArgument.
	ClampNone Clamp = iota
	// ClampBefore moves before the character or tab in text order. On a
	// right-to-left line that is toward the right of the screen.
	ClampBefore
	// ClampAfter moves after the character or tab in text order.
	ClampAfter
)

func (p Position) wire() *pb.Position {
	return &pb.Position{Line: p.Line, Column: p.Column, Character: p.Character, Clamp: pb.Clamp(p.Clamp)}
}

func (p Position) exact() bool { return p.Clamp == ClampNone && p.Character == 0 }

func (p Position) after(other Position) bool {
	return p.Line > other.Line || (p.Line == other.Line && p.Column > other.Column)
}

// Selection never has Start after End. Equal positions are a caret.
type Selection struct{ Start, End Position }

// State is an owned snapshot. Revision begins at zero and is local to EditorID.
// Selection is the newest selection; moving it does not change Revision.
type State struct {
	EditorID, Revision uint64
	Text               string
	ReadOnly           bool
	Selection          Selection
}
type EventKind uint8

const (
	Changed EventKind = iota + 1
	EditorClosed
	Failed
)

type Event struct {
	Sequence, EditorID uint64
	Kind               EventKind
	State              *State
	Error              *ProtocolError
}

// Client supports concurrent commands and a single ordered event consumer.
// Handles belong to this client and the lifetime of its host process.
type Client struct {
	transport Transport
	ids       atomic.Uint64
	caps      Capabilities
	eventsMu  sync.Mutex
	sequence  uint64
}

func NewClient(ctx context.Context, transport Transport) (*Client, error) {
	if transport == nil {
		return nil, errors.New("editor: nil transport")
	}
	c := &Client{transport: transport}
	response, err := c.command(ctx, &pb.Request{Command: &pb.Request_Hello{Hello: &pb.Hello{Major: 1}}})
	if err != nil {
		return nil, err
	}
	caps := response.GetCapabilities()
	if response.EditorId != 0 || caps == nil || caps.Major != 1 || caps.MaxTextBytes == 0 || caps.MaxTextBytes > MaxTextBytes || caps.MaxEditors == 0 {
		return nil, errors.New("editor: invalid capabilities")
	}
	c.caps = Capabilities{Major: caps.Major, MaxTextBytes: caps.MaxTextBytes, MaxEditors: caps.MaxEditors}
	return c, nil
}
func (c *Client) Capabilities() Capabilities { return c.caps }
func (c *Client) exchange(ctx context.Context, operation Operation, request, response proto.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if proto.Size(request) > maxMessageBytes {
		return &ProtocolError{Code: ResourceExhausted, Message: "request exceeds message limit"}
	}
	data, err := proto.Marshal(request)
	if err != nil {
		return err
	}
	data, err = c.transport.Call(ctx, operation, data)
	if err != nil {
		return err
	}
	if len(data) > maxMessageBytes {
		return errors.New("editor: response exceeds message limit")
	}
	return (proto.UnmarshalOptions{RecursionLimit: 32}).Unmarshal(data, response)
}
func (c *Client) command(ctx context.Context, request *pb.Request) (*pb.Response, error) {
	request.RequestId = c.ids.Add(1)
	response := new(pb.Response)
	if err := c.exchange(ctx, OperationCommand, request, response); err != nil {
		return nil, err
	}
	if response.RequestId != request.RequestId {
		return nil, errors.New("editor: response ID mismatch")
	}
	_, opening := request.Command.(*pb.Request_Open)
	_, embedding := request.Command.(*pb.Request_OpenEmbedded)
	if !opening && !embedding && response.EditorId != request.EditorId {
		return nil, errors.New("editor: response editor ID mismatch")
	}
	if failure, ok := response.Result.(*pb.Response_Error); ok {
		if failure.Error == nil {
			return nil, errors.New("editor: missing error details")
		}
		return nil, protocolError(failure.Error)
	}
	return response, nil
}
func (c *Client) checkText(text string) error {
	if !utf8.ValidString(text) {
		return &ProtocolError{Code: InvalidArgument, Message: "text is not UTF-8"}
	}
	if len(text) > int(c.caps.MaxTextBytes) {
		return &ProtocolError{Code: ResourceExhausted, Message: "text exceeds bridge limit"}
	}
	return nil
}
func (c *Client) snapshot(value *pb.State, id uint64) (*State, error) {
	if value == nil || id == 0 || value.EditorId != id {
		return nil, errors.New("editor: missing or mismatched state")
	}
	if err := c.checkText(value.Text); err != nil {
		return nil, err
	}
	selection := value.GetSelection()
	return &State{EditorID: id, Revision: value.Revision, Text: value.Text, ReadOnly: value.ReadOnly, Selection: Selection{
		Start: Position{Line: selection.GetStart().GetLine(), Column: selection.GetStart().GetColumn()},
		End:   Position{Line: selection.GetEnd().GetLine(), Column: selection.GetEnd().GetColumn()},
	}}, nil
}
func (c *Client) Open(ctx context.Context, options Options) (*Editor, error) {
	if err := c.checkText(options.Text); err != nil {
		return nil, err
	}
	if !utf8.ValidString(options.Title) || len(options.Title) > 1024 {
		return nil, &ProtocolError{Code: InvalidArgument, Message: "invalid window title"}
	}
	if options.Title == "" {
		options.Title = "Editor"
	}
	if options.Width == 0 {
		options.Width = 1000
	}
	if options.Height == 0 {
		options.Height = 700
	}
	for _, dimension := range []float32{options.Width, options.Height} {
		if math.IsNaN(float64(dimension)) || math.IsInf(float64(dimension), 0) || dimension < 100 || dimension > 16384 {
			return nil, &ProtocolError{Code: InvalidArgument, Message: "invalid window dimensions"}
		}
	}
	response, err := c.command(ctx, &pb.Request{Command: &pb.Request_Open{Open: &pb.Open{Title: options.Title, Width: options.Width, Height: options.Height, Text: options.Text, ReadOnly: options.ReadOnly, ShowGutter: !options.HideGutter, SoftWrap: !options.DisableSoftWrap, WidthTable: pb.WidthTable(options.WidthTable)}}})
	if err != nil {
		return nil, err
	}
	if _, err := c.snapshot(response.GetState(), response.EditorId); err != nil {
		return nil, err
	}
	return &Editor{client: c, id: response.EditorId}, nil
}

// EmbedOptions configure an editor that is placed inside a generic view.
type EmbedOptions struct {
	Text            string
	ReadOnly        bool
	HideGutter      bool
	DisableSoftWrap bool
	// WidthTable chooses how a Position counts East Asian Ambiguous characters.
	// It does not change how they are drawn.
	WidthTable WidthTable
}

// EmbedTarget is implemented by client.Window. This package has no dependency
// on the generic client.
type EmbedTarget interface {
	EmbedTarget() (view uint64, generation uint32)
}

// OpenEmbedded creates an editor in target's window without showing it. Pass
// its ID to scene.Embedded, in a view committed after the window has opened.
// The editor closes with the window. Use a Client from Attach.
func (c *Client) OpenEmbedded(ctx context.Context, target EmbedTarget, options EmbedOptions) (*Editor, error) {
	if err := c.checkText(options.Text); err != nil {
		return nil, err
	}
	view, generation := target.EmbedTarget()
	response, err := c.command(ctx, &pb.Request{Command: &pb.Request_OpenEmbedded{OpenEmbedded: &pb.OpenEmbedded{
		ViewId: view, ViewGeneration: generation, Text: options.Text, ReadOnly: options.ReadOnly,
		ShowGutter: !options.HideGutter, SoftWrap: !options.DisableSoftWrap, WidthTable: pb.WidthTable(options.WidthTable),
	}}})
	if err != nil {
		return nil, err
	}
	if _, err := c.snapshot(response.GetState(), response.EditorId); err != nil {
		return nil, err
	}
	return &Editor{client: c, id: response.EditorId}, nil
}

func (c *Client) Quit(ctx context.Context) error {
	response, err := c.command(ctx, &pb.Request{Command: &pb.Request_Quit{Quit: &pb.Quit{}}})
	if err == nil && response.GetAck() == nil {
		err = errors.New("editor: missing quit acknowledgement")
	}
	return err
}

// NextEvent consumes one event across all editors. Cancellation can race receipt
// and lose an event; no replay/retry guarantee is provided by this protocol.
func (c *Client) NextEvent(ctx context.Context) (*Event, error) {
	if !c.eventsMu.TryLock() {
		return nil, ErrConcurrentEvents
	}
	defer c.eventsMu.Unlock()
	response := new(pb.Event)
	if err := c.exchange(ctx, OperationEvents, &pb.NextEvent{}, response); err != nil {
		return nil, err
	}
	if response.Sequence <= c.sequence || response.EditorId == 0 {
		return nil, errors.New("editor: invalid event sequence or handle")
	}
	event := &Event{Sequence: response.Sequence, EditorID: response.EditorId}
	switch kind := response.Kind.(type) {
	case *pb.Event_Changed:
		state, err := c.snapshot(kind.Changed, event.EditorID)
		if err != nil {
			return nil, err
		}
		event.Kind, event.State = Changed, state
	case *pb.Event_Closed:
		if kind.Closed == nil {
			return nil, errors.New("editor: missing close event")
		}
		event.Kind = EditorClosed
	case *pb.Event_Error:
		if kind.Error == nil {
			return nil, errors.New("editor: missing event error")
		}
		event.Kind, event.Error = Failed, protocolError(kind.Error)
	default:
		return nil, errors.New("editor: missing event kind")
	}
	c.sequence = response.Sequence
	return event, nil
}

// Editor identifies one persistent native buffer. Closing it invalidates the
// handle; IDs are never recycled. Closing an editor does not quit the host.
type Editor struct {
	client *Client
	id     uint64
}

func (e *Editor) ID() uint64 { return e.id }
func (e *Editor) stateCommand(ctx context.Context, request *pb.Request) (*State, error) {
	request.EditorId = e.id
	response, err := e.client.command(ctx, request)
	if err != nil {
		return nil, err
	}
	return e.client.snapshot(response.GetState(), e.id)
}
func (e *Editor) State(ctx context.Context) (*State, error) {
	return e.stateCommand(ctx, &pb.Request{Command: &pb.Request_GetState{GetState: &pb.GetState{}}})
}

// SetText is an authoritative replacement, also allowed in read-only mode.
func (e *Editor) SetText(ctx context.Context, text string) (*State, error) {
	if err := e.client.checkText(text); err != nil {
		return nil, err
	}
	return e.stateCommand(ctx, &pb.Request{Command: &pb.Request_SetText{SetText: &pb.SetText{Text: text}}})
}

// InsertText replaces the current native selections in one undo transaction.
func (e *Editor) InsertText(ctx context.Context, text string) (*State, error) {
	if err := e.client.checkText(text); err != nil {
		return nil, err
	}
	return e.stateCommand(ctx, &pb.Request{Command: &pb.Request_InsertText{InsertText: &pb.InsertText{Text: text}}})
}
func (e *Editor) Undo(ctx context.Context) (*State, error) {
	return e.stateCommand(ctx, &pb.Request{Command: &pb.Request_Undo{Undo: &pb.Undo{}}})
}
func (e *Editor) Redo(ctx context.Context) (*State, error) {
	return e.stateCommand(ctx, &pb.Request{Command: &pb.Request_Redo{Redo: &pb.Redo{}}})
}
func (e *Editor) SetReadOnly(ctx context.Context, value bool) (*State, error) {
	return e.stateCommand(ctx, &pb.Request{Command: &pb.Request_SetReadOnly{SetReadOnly: &pb.SetReadOnly{Value: value}}})
}

// SetSelection replaces all selections with one from start to end and scrolls
// it into view. Pass the same position twice for a caret. It is also allowed on
// a read-only editor, does not move keyboard focus and emits no Changed event.
// A position outside the buffer returns InvalidArgument, and so does one inside
// a tab or character unless its Clamp says which way to move. Clamp each end
// separately: the start before and the end after selects the whole character.
func (e *Editor) SetSelection(ctx context.Context, start, end Position) (*State, error) {
	// Only native code knows where clamped or character-counted positions land.
	if start.exact() && end.exact() && start.after(end) {
		return nil, &ProtocolError{Code: InvalidArgument, Message: "selection start is after its end"}
	}
	return e.stateCommand(ctx, &pb.Request{Command: &pb.Request_SetSelection{SetSelection: &pb.SetSelection{Selection: &pb.Selection{
		Start: start.wire(),
		End:   end.wire(),
	}}}})
}
func (e *Editor) ackCommand(ctx context.Context, request *pb.Request) error {
	request.EditorId = e.id
	response, err := e.client.command(ctx, request)
	if err == nil && response.GetAck() == nil {
		err = errors.New("editor: missing acknowledgement")
	}
	return err
}
func (e *Editor) Focus(ctx context.Context) error {
	return e.ackCommand(ctx, &pb.Request{Command: &pb.Request_Focus{Focus: &pb.Focus{}}})
}
func (e *Editor) Close(ctx context.Context) error {
	return e.ackCommand(ctx, &pb.Request{Command: &pb.Request_Close{Close: &pb.Close{}}})
}
