package scene

import pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"

// Snapshot values are aliases of the wire types, preserving protobuf methods and identity.
type (
	Handle       = pb.Handle
	Point        = pb.Point
	Size         = pb.Size
	Bounds       = pb.Bounds
	Length       = pb.Length
	Color        = pb.Color
	Edges        = pb.Edges
	Modifiers    = pb.Modifiers
	Element      = pb.Element
	EventBinding = pb.EventBinding
	TextChanged  = pb.TextChanged
)

// Style values retain optional-field presence and generated enum methods.
type (
	Style            = pb.Style
	Shadow           = pb.Shadow
	OpacityAnimation = pb.OpacityAnimation
	FlexDirection    = pb.FlexDirection
	Alignment        = pb.Alignment
	Justification    = pb.Justification
	Overflow         = pb.Overflow
	Cursor           = pb.Cursor
	Position         = pb.Position
	Display          = pb.Display
	TextAlignment    = pb.TextAlignment
)

// Interaction and accessibility values configure native behavior declaratively.
type (
	Interaction    = pb.Interaction
	Accessibility  = pb.Accessibility
	AccessibleRole = pb.AccessibleRole
	DragData       = pb.DragData
	Action         = pb.Action
	KeyBinding     = pb.KeyBinding
	FocusTarget    = pb.FocusTarget
)

// Drawing values include the exported oneof cases used to construct commands.
type (
	DrawCommand     = pb.DrawCommand
	DrawRect        = pb.DrawRect
	DrawPath        = pb.DrawPath
	PathStep        = pb.PathStep
	PathVerb        = pb.PathVerb
	Drawing         = pb.Drawing
	DrawCommandRect = pb.DrawCommand_Rect
	DrawCommandPath = pb.DrawCommand_Path
)

// FlexDirection values use concise application-facing names.
const (
	FlexUnspecified = pb.FlexDirection_FLEX_DIRECTION_UNSPECIFIED
	FlexRow         = pb.FlexDirection_FLEX_DIRECTION_ROW
	FlexColumn      = pb.FlexDirection_FLEX_DIRECTION_COLUMN
)

// Alignment values use concise application-facing names.
const (
	AlignUnspecified = pb.Alignment_ALIGNMENT_UNSPECIFIED
	AlignStart       = pb.Alignment_ALIGNMENT_START
	AlignCenter      = pb.Alignment_ALIGNMENT_CENTER
	AlignEnd         = pb.Alignment_ALIGNMENT_END
	AlignStretch     = pb.Alignment_ALIGNMENT_STRETCH
)

// Justification values use concise application-facing names.
const (
	JustifyUnspecified  = pb.Justification_JUSTIFICATION_UNSPECIFIED
	JustifyStart        = pb.Justification_JUSTIFICATION_START
	JustifyCenter       = pb.Justification_JUSTIFICATION_CENTER
	JustifyEnd          = pb.Justification_JUSTIFICATION_END
	JustifySpaceBetween = pb.Justification_JUSTIFICATION_SPACE_BETWEEN
)

// Overflow values use concise application-facing names.
const (
	OverflowUnspecified = pb.Overflow_OVERFLOW_UNSPECIFIED
	OverflowVisible     = pb.Overflow_OVERFLOW_VISIBLE
	OverflowHidden      = pb.Overflow_OVERFLOW_HIDDEN
	OverflowScroll      = pb.Overflow_OVERFLOW_SCROLL
)

// Cursor values use concise application-facing names.
const (
	CursorUnspecified = pb.Cursor_CURSOR_UNSPECIFIED
	CursorArrow       = pb.Cursor_CURSOR_ARROW
	CursorPointer     = pb.Cursor_CURSOR_POINTER
)

// Position values use concise application-facing names.
const (
	PositionUnspecified = pb.Position_POSITION_UNSPECIFIED
	PositionRelative    = pb.Position_POSITION_RELATIVE
	PositionAbsolute    = pb.Position_POSITION_ABSOLUTE
)

// Display values use concise application-facing names.
const (
	DisplayUnspecified = pb.Display_DISPLAY_UNSPECIFIED
	DisplayFlex        = pb.Display_DISPLAY_FLEX
	DisplayGrid        = pb.Display_DISPLAY_GRID
	DisplayNone        = pb.Display_DISPLAY_NONE
)

// TextAlignment values use concise application-facing names.
const (
	TextAlignUnspecified = pb.TextAlignment_TEXT_ALIGNMENT_UNSPECIFIED
	TextAlignLeft        = pb.TextAlignment_TEXT_ALIGNMENT_LEFT
	TextAlignCenter      = pb.TextAlignment_TEXT_ALIGNMENT_CENTER
	TextAlignRight       = pb.TextAlignment_TEXT_ALIGNMENT_RIGHT
)

// AccessibleRole values use concise application-facing names.
const (
	RoleUnspecified = pb.AccessibleRole_ACCESSIBLE_ROLE_UNSPECIFIED
	RoleGeneric     = pb.AccessibleRole_ACCESSIBLE_ROLE_GENERIC
	RoleButton      = pb.AccessibleRole_ACCESSIBLE_ROLE_BUTTON
	RoleCheckbox    = pb.AccessibleRole_ACCESSIBLE_ROLE_CHECKBOX
	RoleTextInput   = pb.AccessibleRole_ACCESSIBLE_ROLE_TEXT_INPUT
	RoleText        = pb.AccessibleRole_ACCESSIBLE_ROLE_TEXT
	RoleList        = pb.AccessibleRole_ACCESSIBLE_ROLE_LIST
	RoleListItem    = pb.AccessibleRole_ACCESSIBLE_ROLE_LIST_ITEM
	RoleLink        = pb.AccessibleRole_ACCESSIBLE_ROLE_LINK
	RoleImage       = pb.AccessibleRole_ACCESSIBLE_ROLE_IMAGE
	RoleSlider      = pb.AccessibleRole_ACCESSIBLE_ROLE_SLIDER
	RoleDialog      = pb.AccessibleRole_ACCESSIBLE_ROLE_DIALOG
)

// PathVerb values use concise application-facing names.
const (
	PathUnspecified = pb.PathVerb_PATH_VERB_UNSPECIFIED
	PathMove        = pb.PathVerb_PATH_VERB_MOVE
	PathLine        = pb.PathVerb_PATH_VERB_LINE
	PathQuadratic   = pb.PathVerb_PATH_VERB_QUADRATIC
	PathCubic       = pb.PathVerb_PATH_VERB_CUBIC
	PathClose       = pb.PathVerb_PATH_VERB_CLOSE
)
