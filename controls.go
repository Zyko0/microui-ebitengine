// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 The Ebitengine Authors

package microui

import (
	"fmt"
	"image"
	"math"
	"strconv"
	"unsafe"
)

func (c *Context) inHoverRoot() bool {
	for i := len(c.containerStack) - 1; i >= 0; i-- {
		if c.containerStack[i] == c.hoverRoot {
			return true
		}
		// only root containers have their `head` field set; stop searching if we've
		// reached the current root container
		if c.containerStack[i].HeadIdx >= 0 {
			break
		}
	}
	return false
}

func (c *Context) drawControlFrame(id ID, rect image.Rectangle, colorid int, opt Option) {
	if (opt & OptNoFrame) != 0 {
		return
	}
	if c.focus == id {
		colorid += 2
	} else if c.hover == id {
		colorid++
	}
	c.drawFrame(rect, colorid)
}

func (c *Context) drawControlText(str string, rect image.Rectangle, colorid int, opt Option) {
	var pos image.Point
	tw := textWidth(str)
	c.pushClipRect(rect)
	pos.Y = rect.Min.Y + (rect.Dy()-lineHeight())/2
	if (opt & OptAlignCenter) != 0 {
		pos.X = rect.Min.X + (rect.Dx()-tw)/2
	} else if (opt & OptAlignRight) != 0 {
		pos.X = rect.Min.X + rect.Dx() - tw - c.Style.Padding
	} else {
		pos.X = rect.Min.X + c.Style.Padding
	}
	c.drawText(str, pos, c.Style.Colors[colorid])
	c.popClipRect()
}

func (c *Context) mouseOver(rect image.Rectangle) bool {
	return c.mousePos.In(rect) && c.mousePos.In(c.clipRect()) && c.inHoverRoot()
}

func (c *Context) updateControl(id ID, rect image.Rectangle, opt Option) {
	if id == 0 {
		return
	}

	mouseover := c.mouseOver(rect)

	if c.focus == id {
		c.keepFocus = true
	}
	if (opt & OptNoInteract) != 0 {
		return
	}
	if mouseover && c.mouseDown == 0 {
		c.hover = id
	}

	if c.focus == id {
		if c.mousePressed != 0 && !mouseover {
			c.SetFocus(0)
		}
		if c.mouseDown == 0 && (^opt&OptHoldFocus) != 0 {
			c.SetFocus(0)
		}
	}

	if c.hover == id {
		if c.mousePressed != 0 {
			c.SetFocus(id)
		} else if !mouseover {
			c.hover = 0
		}
	}
}

func (c *Context) Control(id ID, opt Option, f func(r image.Rectangle) Response) Response {
	r := c.layoutNext()
	c.updateControl(id, r, opt)
	return f(r)
}

func (c *Context) Text(text string) {
	color := c.Style.Colors[ColorText]
	c.LayoutColumn(func() {
		var endIdx, p int
		c.SetLayoutRow([]int{-1}, lineHeight())
		for endIdx < len(text) {
			c.Control(0, 0, func(r image.Rectangle) Response {
				w := 0
				endIdx = p
				startIdx := endIdx
				for endIdx < len(text) && text[endIdx] != '\n' {
					word := p
					for p < len(text) && text[p] != ' ' && text[p] != '\n' {
						p++
					}
					w += textWidth(text[word:p])
					if w > r.Dx() && endIdx != startIdx {
						break
					}
					if p < len(text) {
						w += textWidth(string(text[p]))
					}
					endIdx = p
					p++
				}
				c.drawText(text[startIdx:endIdx], r.Min, color)
				p = endIdx + 1
				return 0
			})
		}
	})
}

func (c *Context) Label(text string) {
	c.Control(0, 0, func(r image.Rectangle) Response {
		c.drawControlText(text, r, ColorText, 0)
		return 0
	})
}

func (c *Context) buttonEx(label string, opt Option) Response {
	var id ID
	if len(label) > 0 {
		id = c.id([]byte(label))
	}
	return c.Control(id, opt, func(r image.Rectangle) Response {
		var res Response
		// handle click
		if c.mousePressed == mouseLeft && c.focus == id {
			res |= ResponseSubmit
		}
		// draw
		c.drawControlFrame(id, r, ColorButton, opt)
		if len(label) > 0 {
			c.drawControlText(label, r, ColorText, opt)
		}
		return res
	})
}

func (c *Context) Checkbox(label string, state *bool) Response {
	id := c.id(ptrToBytes(unsafe.Pointer(state)))
	return c.Control(id, 0, func(r image.Rectangle) Response {
		var res Response
		box := image.Rect(r.Min.X, r.Min.Y, r.Min.X+r.Dy(), r.Max.Y)
		c.updateControl(id, r, 0)
		// handle click
		if c.mousePressed == mouseLeft && c.focus == id {
			res |= ResponseChange
			*state = !*state
		}
		// draw
		c.drawControlFrame(id, box, ColorBase, 0)
		if *state {
			c.drawIcon(iconCheck, box, c.Style.Colors[ColorText])
		}
		r = image.Rect(r.Min.X+box.Dx(), r.Min.Y, r.Max.X, r.Max.Y)
		c.drawControlText(label, r, ColorText, 0)
		return res
	})
}

func (c *Context) textBoxRaw(buf *string, id ID, opt Option) Response {
	return c.Control(id, opt|OptHoldFocus, func(r image.Rectangle) Response {
		var res Response
		buflen := len(*buf)

		if c.focus == id {
			// handle text input
			if len(c.textInput) > 0 {
				*buf += string(c.textInput)
				res |= ResponseChange
			}
			// handle backspace
			if (c.keyPressed&keyBackspace) != 0 && buflen > 0 {
				*buf = (*buf)[:buflen-1]
				res |= ResponseChange
			}
			// handle return
			if (c.keyPressed & keyReturn) != 0 {
				c.SetFocus(0)
				res |= ResponseSubmit
			}
		}

		// draw
		c.drawControlFrame(id, r, ColorBase, opt)
		if c.focus == id {
			color := c.Style.Colors[ColorText]
			textw := textWidth(*buf)
			texth := lineHeight()
			ofx := r.Dx() - c.Style.Padding - textw - 1
			textx := r.Min.X + min(ofx, c.Style.Padding)
			texty := r.Min.Y + (r.Dy()-texth)/2
			c.pushClipRect(r)
			c.drawText(*buf, image.Pt(textx, texty), color)
			c.drawRect(image.Rect(textx+textw, texty, textx+textw+1, texty+texth), color)
			c.popClipRect()
		} else {
			c.drawControlText(*buf, r, ColorText, opt)
		}
		return res
	})
}

func (c *Context) numberTextBox(value *float64, id ID) bool {
	if c.mousePressed == mouseLeft && (c.keyDown&keyShift) != 0 &&
		c.hover == id {
		c.numberEdit = id
		c.numberEditBuf = fmt.Sprintf(realFmt, *value)
	}
	if c.numberEdit == id {
		res := c.textBoxRaw(&c.numberEditBuf, id, 0)
		if (res&ResponseSubmit) != 0 || c.focus != id {
			nval, err := strconv.ParseFloat(c.numberEditBuf, 32)
			if err != nil {
				nval = 0
			}
			*value = float64(nval)
			c.numberEdit = 0
		}
		return true
	}
	return false
}

func (c *Context) textBoxEx(buf *string, opt Option) Response {
	id := c.id(ptrToBytes(unsafe.Pointer(buf)))
	return c.textBoxRaw(buf, id, opt)
}

func (c *Context) SliderEx(value *float64, low, high, step float64, format string, opt Option) Response {
	last := *value
	v := last
	id := c.id(ptrToBytes(unsafe.Pointer(value)))

	// handle text input mode
	if c.numberTextBox(&v, id) {
		return 0
	}

	// handle normal mode
	return c.Control(id, opt, func(r image.Rectangle) Response {
		var res Response
		// handle input
		if c.focus == id && (c.mouseDown|c.mousePressed) == mouseLeft {
			v = low + float64(c.mousePos.X-r.Min.X)*(high-low)/float64(r.Dx())
			if step != 0 {
				v = math.Round(v/step) * step
			}
		}
		// clamp and store value, update res
		*value = clampF(v, low, high)
		v = *value
		if last != v {
			res |= ResponseChange
		}

		// draw base
		c.drawControlFrame(id, r, ColorBase, opt)
		// draw thumb
		w := c.Style.ThumbSize
		x := int((v - low) * float64(r.Dx()-w) / (high - low))
		thumb := image.Rect(r.Min.X+x, r.Min.Y, r.Min.X+x+w, r.Max.Y)
		c.drawControlFrame(id, thumb, ColorButton, opt)
		// draw text
		text := fmt.Sprintf(format, v)
		c.drawControlText(text, r, ColorText, opt)

		return res
	})
}

func (c *Context) NumberEx(value *float64, step float64, format string, opt Option) Response {
	id := c.id(ptrToBytes(unsafe.Pointer(value)))
	last := *value

	// handle text input mode
	if c.numberTextBox(value, id) {
		return 0
	}

	// handle normal mode
	return c.Control(id, opt, func(r image.Rectangle) Response {
		var res Response
		// handle input
		if c.focus == id && c.mouseDown == mouseLeft {
			*value += float64(c.mouseDelta.X) * step
		}
		// set flag if value changed
		if *value != last {
			res |= ResponseChange
		}

		// draw base
		c.drawControlFrame(id, r, ColorBase, opt)
		// draw text
		text := fmt.Sprintf(format, *value)
		c.drawControlText(text, r, ColorText, opt)

		return res
	})
}

func (c *Context) header(label string, istreenode bool, opt Option) Response {
	id := c.id([]byte(label))
	idx := c.poolGet(c.treeNodePool[:], id)
	c.SetLayoutRow([]int{-1}, 0)

	active := idx >= 0
	var expanded bool
	if (opt & OptExpanded) != 0 {
		expanded = !active
	} else {
		expanded = active
	}

	return c.Control(id, 0, func(r image.Rectangle) Response {
		// handle click (TODO (port): check if this is correct)
		clicked := c.mousePressed == mouseLeft && c.focus == id
		v1, v2 := 0, 0
		if active {
			v1 = 1
		}
		if clicked {
			v2 = 1
		}
		active = (v1 ^ v2) == 1

		// update pool ref
		if idx >= 0 {
			if active {
				c.poolUpdate(c.treeNodePool[:], idx)
			} else {
				c.treeNodePool[idx] = poolItem{}
			}
		} else if active {
			c.poolInit(c.treeNodePool[:], id)
		}

		// draw
		if istreenode {
			if c.hover == id {
				c.drawFrame(r, ColorButtonHover)
			}
		} else {
			c.drawControlFrame(id, r, ColorButton, 0)
		}
		var icon icon
		if expanded {
			icon = iconExpanded
		} else {
			icon = iconCollapsed
		}
		c.drawIcon(
			icon,
			image.Rect(r.Min.X, r.Min.Y, r.Min.X+r.Dy(), r.Max.Y),
			c.Style.Colors[ColorText],
		)
		r.Min.X += r.Dy() - c.Style.Padding
		c.drawControlText(label, r, ColorText, 0)

		if expanded {
			return ResponseActive
		}
		return 0
	})
}

func (c *Context) HeaderEx(label string, opt Option) Response {
	return c.header(label, false, opt)
}

func (c *Context) treeNode(label string, opt Option, f func(res Response)) {
	res := c.header(label, true, opt)
	if res&ResponseActive == 0 {
		return
	}
	c.layout().indent += c.Style.Indent
	defer func() {
		c.layout().indent -= c.Style.Indent
	}()
	c.idStack = append(c.idStack, c.LastID)
	defer c.popID()
	f(res)
}

// x = x, y = y, w = w, h = h
func (c *Context) scrollbarVertical(cnt *Container, b image.Rectangle, cs image.Point) {
	maxscroll := cs.Y - b.Dy()
	if maxscroll > 0 && b.Dy() > 0 {
		id := c.id([]byte("!scrollbar" + "y"))

		// get sizing / positioning
		base := b
		base.Min.X = b.Max.X
		base.Max.X = base.Min.X + c.Style.ScrollbarSize

		// handle input
		c.updateControl(id, base, 0)
		if c.focus == id && c.mouseDown == mouseLeft {
			cnt.Scroll.Y += c.mouseDelta.Y * cs.Y / base.Dy()
		}
		// clamp scroll to limits
		cnt.Scroll.Y = clamp(cnt.Scroll.Y, 0, maxscroll)

		// draw base and thumb
		c.drawFrame(base, ColorScrollBase)
		thumb := base
		thumb.Max.Y = thumb.Min.Y + max(c.Style.ThumbSize, base.Dy()*b.Dy()/cs.Y)
		thumb = thumb.Add(image.Pt(0, cnt.Scroll.Y*(base.Dy()-thumb.Dy())/maxscroll))
		c.drawFrame(thumb, ColorScrollThumb)

		// set this as the scroll_target (will get scrolled on mousewheel)
		// if the mouse is over it
		if c.mouseOver(b) {
			c.scrollTarget = cnt
		}
	} else {
		cnt.Scroll.Y = 0
	}
}

// x = y, y = x, w = h, h = w
func (c *Context) scrollbarHorizontal(cnt *Container, b image.Rectangle, cs image.Point) {
	maxscroll := cs.X - b.Dx()
	if maxscroll > 0 && b.Dx() > 0 {
		id := c.id([]byte("!scrollbar" + "x"))

		// get sizing / positioning
		base := b
		base.Min.Y = b.Max.Y
		base.Max.Y = base.Min.Y + c.Style.ScrollbarSize

		// handle input
		c.updateControl(id, base, 0)
		if c.focus == id && c.mouseDown == mouseLeft {
			cnt.Scroll.X += c.mouseDelta.X * cs.X / base.Dx()
		}
		// clamp scroll to limits
		cnt.Scroll.X = clamp(cnt.Scroll.X, 0, maxscroll)

		// draw base and thumb
		c.drawFrame(base, ColorScrollBase)
		thumb := base
		thumb.Max.X = thumb.Min.X + max(c.Style.ThumbSize, base.Dx()*b.Dx()/cs.X)
		thumb = thumb.Add(image.Pt(cnt.Scroll.X*(base.Dx()-thumb.Dx())/maxscroll, 0))
		c.drawFrame(thumb, ColorScrollThumb)

		// set this as the scroll_target (will get scrolled on mousewheel)
		// if the mouse is over it
		if c.mouseOver(b) {
			c.scrollTarget = cnt
		}
	} else {
		cnt.Scroll.X = 0
	}
}

// if `swap` is true, X = Y, Y = X, W = H, H = W
func (c *Context) scrollbar(cnt *Container, b image.Rectangle, cs image.Point, swap bool) {
	if swap {
		c.scrollbarHorizontal(cnt, b, cs)
	} else {
		c.scrollbarVertical(cnt, b, cs)
	}
}

func (c *Context) scrollbars(cnt *Container, body image.Rectangle) image.Rectangle {
	sz := c.Style.ScrollbarSize
	cs := cnt.ContentSize
	cs.X += c.Style.Padding * 2
	cs.Y += c.Style.Padding * 2
	c.pushClipRect(body)
	// resize body to make room for scrollbars
	if cs.Y > cnt.Body.Dy() {
		body.Max.X -= sz
	}
	if cs.X > cnt.Body.Dx() {
		body.Max.Y -= sz
	}
	// to create a horizontal or vertical scrollbar almost-identical code is
	// used; only the references to `x|y` `w|h` need to be switched
	c.scrollbar(cnt, body, cs, false)
	c.scrollbar(cnt, body, cs, true)
	c.popClipRect()
	return body
}

func (c *Context) pushContainerBody(cnt *Container, body image.Rectangle, opt Option) {
	if (^opt & OptNoScroll) != 0 {
		body = c.scrollbars(cnt, body)
	}
	c.pushLayout(body.Inset(c.Style.Padding), cnt.Scroll)
	cnt.Body = body
}

func (c *Context) window(title string, rect image.Rectangle, opt Option, f func(res Response)) {
	id := c.id([]byte(title))

	cnt := c.container(id, opt)
	if cnt == nil || !cnt.Open {
		return
	}
	c.idStack = append(c.idStack, id)
	defer c.popID()
	// This is popped at endRootContainer.
	// TODO: This is tricky. Refactor this.

	if cnt.Rect.Dx() == 0 {
		cnt.Rect = rect
	}

	c.containerStack = append(c.containerStack, cnt)
	defer c.popContainer()

	// push container to roots list and push head command
	c.rootList = append(c.rootList, cnt)
	cnt.HeadIdx = c.pushJump(-1)
	defer func() {
		// push tail 'goto' jump command and set head 'skip' command. the final steps
		// on initing these are done in End
		cnt := c.CurrentContainer()
		cnt.TailIdx = c.pushJump(-1)
		c.commandList[cnt.HeadIdx].jump.dstIdx = len(c.commandList) //- 1
	}()

	// set as hover root if the mouse is overlapping this container and it has a
	// higher zindex than the current hover root
	if c.mousePos.In(cnt.Rect) && (c.nextHoverRoot == nil || cnt.ZIndex > c.nextHoverRoot.ZIndex) {
		c.nextHoverRoot = cnt
	}

	// clipping is reset here in case a root-container is made within
	// another root-containers's begin/end block; this prevents the inner
	// root-container being clipped to the outer
	c.clipStack = append(c.clipStack, unclippedRect)
	defer c.popClipRect()

	body := cnt.Rect
	rect = body

	// draw frame
	if (^opt & OptNoFrame) != 0 {
		c.drawFrame(rect, ColorWindowBG)
	}

	// do title bar
	if (^opt & OptNoTitle) != 0 {
		tr := rect
		tr.Max.Y = tr.Min.Y + c.Style.TitleHeight
		c.drawFrame(tr, ColorTitleBG)

		// do title text
		if (^opt & OptNoTitle) != 0 {
			id := c.id([]byte("!title"))
			c.updateControl(id, tr, opt)
			c.drawControlText(title, tr, ColorTitleText, opt)
			if id == c.focus && c.mouseDown == mouseLeft {
				cnt.Rect = cnt.Rect.Add(c.mouseDelta)
			}
			body.Min.Y += tr.Dy()
		}

		// do `close` button
		if (^opt & OptNoClose) != 0 {
			id := c.id([]byte("!close"))
			r := image.Rect(tr.Max.X-tr.Dy(), tr.Min.Y, tr.Max.X, tr.Max.Y)
			tr.Max.X -= r.Dx()
			c.drawIcon(iconClose, r, c.Style.Colors[ColorTitleText])
			c.updateControl(id, r, opt)
			if c.mousePressed == mouseLeft && id == c.focus {
				cnt.Open = false
			}
		}
	}

	c.pushContainerBody(cnt, body, opt)

	// do `resize` handle
	if (^opt & OptNoResize) != 0 {
		sz := c.Style.TitleHeight
		id := c.id([]byte("!resize"))
		r := image.Rect(rect.Max.X-sz, rect.Max.Y-sz, rect.Max.X, rect.Max.Y)
		c.updateControl(id, r, opt)
		if id == c.focus && c.mouseDown == mouseLeft {
			cnt.Rect.Max.X = cnt.Rect.Min.X + max(96, cnt.Rect.Dx()+c.mouseDelta.X)
			cnt.Rect.Max.Y = cnt.Rect.Min.Y + max(64, cnt.Rect.Dy()+c.mouseDelta.Y)
		}
	}

	// resize to content size
	if (opt & OptAutoSize) != 0 {
		r := c.layout().body
		cnt.Rect.Max.X = cnt.Rect.Min.X + cnt.ContentSize.X + (cnt.Rect.Dx() - r.Dx())
		cnt.Rect.Max.Y = cnt.Rect.Min.Y + cnt.ContentSize.Y + (cnt.Rect.Dy() - r.Dy())
	}

	// close if this is a popup window and elsewhere was clicked
	if (opt&OptPopup) != 0 && c.mousePressed != 0 && c.hoverRoot != cnt {
		cnt.Open = false
	}

	c.pushClipRect(cnt.Body)
	defer c.popClipRect()

	f(ResponseActive)
}

func (c *Context) OpenPopup(name string) {
	cnt := c.Container(name)
	// set as hover root so popup isn't closed in begin_window_ex()
	c.nextHoverRoot = cnt
	c.hoverRoot = c.nextHoverRoot
	// position at mouse cursor, open and bring-to-front
	cnt.Rect = image.Rect(c.mousePos.X, c.mousePos.Y, c.mousePos.X+1, c.mousePos.Y+1)
	cnt.Open = true
	c.bringToFront(cnt)
}

func (c *Context) Popup(name string, f func(res Response)) {
	opt := OptPopup | OptAutoSize | OptNoResize | OptNoScroll | OptNoTitle | OptClosed
	c.window(name, image.Rectangle{}, opt, f)
}

func (c *Context) panel(name string, opt Option, f func()) {
	id := c.pushID([]byte(name))
	defer c.popID()

	cnt := c.container(id, opt)
	cnt.Rect = c.layoutNext()
	if (^opt & OptNoFrame) != 0 {
		c.drawFrame(cnt.Rect, ColorPanelBG)
	}

	c.containerStack = append(c.containerStack, cnt)
	c.pushContainerBody(cnt, cnt.Rect, opt)
	defer c.popContainer()

	c.pushClipRect(cnt.Body)
	defer c.popClipRect()

	f()
}
