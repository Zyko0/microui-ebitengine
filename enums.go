// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 The Ebitengine Authors

package microui

const (
	clipPart = 1 + iota
	clipAll
)

const (
	commandJump = 1 + iota
	commandClip
	commandRect
	commandText
	commandIcon
	commandDraw
)

const (
	ColorText = iota
	ColorBorder
	ColorWindowBG
	ColorTitleBG
	ColorTitleText
	ColorPanelBG
	ColorButton
	ColorButtonHover
	ColorButtonFocus
	ColorBase
	ColorBaseHover
	ColorBaseFocus
	ColorScrollBase
	ColorScrollThumb
	ColorMax = ColorScrollThumb
)

type icon int

const (
	iconClose icon = 1 + iota
	iconCheck
	iconCollapsed
	iconExpanded
)

type Response int

const (
	ResponseActive Response = (1 << 0)
	ResponseSubmit Response = (1 << 1)
	ResponseChange Response = (1 << 2)
)

type Option int

const (
	OptAlignCenter Option = (1 << iota)
	OptAlignRight
	OptNoInteract
	OptNoFrame
	OptNoResize
	OptNoScroll
	OptNoClose
	OptNoTitle
	OptHoldFocus
	OptAutoSize
	OptPopup
	OptClosed
	OptExpanded
)

const (
	mouseLeft   = (1 << 0)
	mouseRight  = (1 << 1)
	mouseMiddle = (1 << 2)
)

const (
	keyShift     = (1 << 0)
	keyControl   = (1 << 1)
	keyAlt       = (1 << 2)
	keyBackspace = (1 << 3)
	keyReturn    = (1 << 4)
)
