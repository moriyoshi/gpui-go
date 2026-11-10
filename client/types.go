package client

import pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"

// Events and payloads are shared with existing protobuf callers without conversion.
type (
	Event              = pb.Event
	Click              = pb.Click
	KeyEvent           = pb.KeyEvent
	PointerEvent       = pb.PointerEvent
	FocusChanged       = pb.FocusChanged
	FileDrop           = pb.FileDrop
	WindowClosed       = pb.WindowClosed
	WindowResized      = pb.WindowResized
	ColorSchemeChanged = pb.ColorSchemeChanged
	ViewUpdated        = pb.ViewUpdated
	TextChanged        = pb.TextChanged
	MouseButton        = pb.MouseButton
)

// Event kinds expose the wire oneof cases without generated underscore names.
type (
	EventClick              = pb.Event_Click
	EventKeyDown            = pb.Event_KeyDown
	EventAction             = pb.Event_Action
	EventWindowClosed       = pb.Event_WindowClosed
	EventViewUpdated        = pb.Event_ViewUpdated
	EventTextChanged        = pb.Event_TextChanged
	EventKeyUp              = pb.Event_KeyUp
	EventMouseDown          = pb.Event_MouseDown
	EventMouseUp            = pb.Event_MouseUp
	EventMouseMove          = pb.Event_MouseMove
	EventScroll             = pb.Event_Scroll
	EventFocusChanged       = pb.Event_FocusChanged
	EventFileDrop           = pb.Event_FileDrop
	EventModelChanged       = pb.Event_ModelChanged
	EventPathResult         = pb.Event_PathResult
	EventDataDrop           = pb.Event_DataDrop
	EventWindowResized      = pb.Event_WindowResized
	EventColorSchemeChanged = pb.Event_ColorSchemeChanged
)

// Client service values retain their generated accessors and protobuf identity.
type (
	Capabilities    = pb.Capabilities
	ErrorCode       = pb.ErrorCode
	ResourceKind    = pb.ResourceKind
	WindowOperation = pb.WindowOperation
	ModelState      = pb.ModelState
	Menu            = pb.Menu
	MenuItem        = pb.MenuItem
	PathPrompt      = pb.PathPrompt
	SavePathPrompt  = pb.SavePathPrompt
	PathResult      = pb.PathResult
)

// MouseButton values use concise application-facing names.
const (
	MouseUnspecified = pb.MouseButton_MOUSE_BUTTON_UNSPECIFIED
	MouseLeft        = pb.MouseButton_MOUSE_BUTTON_LEFT
	MouseRight       = pb.MouseButton_MOUSE_BUTTON_RIGHT
	MouseMiddle      = pb.MouseButton_MOUSE_BUTTON_MIDDLE
)

// ResourceKind values use concise application-facing names.
const (
	ResourceUnspecified = pb.ResourceKind_RESOURCE_KIND_UNSPECIFIED
	ResourceImage       = pb.ResourceKind_RESOURCE_KIND_IMAGE
	ResourceSVG         = pb.ResourceKind_RESOURCE_KIND_SVG
	ResourceFont        = pb.ResourceKind_RESOURCE_KIND_FONT
)

// WindowOperation values use concise application-facing names.
const (
	WindowUnspecified = pb.WindowOperation_WINDOW_OPERATION_UNSPECIFIED
	WindowActivate    = pb.WindowOperation_WINDOW_OPERATION_ACTIVATE
	WindowMinimize    = pb.WindowOperation_WINDOW_OPERATION_MINIMIZE
	WindowZoom        = pb.WindowOperation_WINDOW_OPERATION_ZOOM
	WindowFullscreen  = pb.WindowOperation_WINDOW_OPERATION_FULLSCREEN
	WindowSetTitle    = pb.WindowOperation_WINDOW_OPERATION_SET_TITLE
	WindowResize      = pb.WindowOperation_WINDOW_OPERATION_RESIZE
)

// ErrorCode values use concise application-facing names.
const (
	ErrorUnspecified     = pb.ErrorCode_ERROR_CODE_UNSPECIFIED
	ErrorInvalidArgument = pb.ErrorCode_ERROR_CODE_INVALID_ARGUMENT
	ErrorUnsupported     = pb.ErrorCode_ERROR_CODE_UNSUPPORTED
	ErrorStaleHandle     = pb.ErrorCode_ERROR_CODE_STALE_HANDLE
	ErrorStaleRevision   = pb.ErrorCode_ERROR_CODE_STALE_REVISION
	ErrorWrongSession    = pb.ErrorCode_ERROR_CODE_WRONG_SESSION
	ErrorClosed          = pb.ErrorCode_ERROR_CODE_CLOSED
	ErrorBackpressure    = pb.ErrorCode_ERROR_CODE_BACKPRESSURE
	ErrorInternal        = pb.ErrorCode_ERROR_CODE_INTERNAL
)
