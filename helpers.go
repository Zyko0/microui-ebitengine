// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 The Ebitengine Authors

package microui

import (
	"image"
	"sort"
	"unsafe"
)

func expect(x bool) {
	if !x {
		panic("expect() failed")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func clamp(x, a, b int) int {
	return min(b, max(a, x))
}

func clampF(x, a, b float64) float64 {
	return minF(b, maxF(a, x))
}

func fnv1a(init ID, data []byte) ID {
	h := init
	for i := 0; i < len(data); i++ {
		h = (h ^ ID(data[i])) * 1099511628211
	}
	return h
}

func ptrToBytes(ptr unsafe.Pointer) []byte {
	slice := unsafe.Slice((*byte)(unsafe.Pointer(&ptr)), unsafe.Sizeof(ptr))

	// `slice` points to `ptr`, which is currently allocated on the stack.
	// after this function returns, `slice` will point to freed memory, so
	// we need to copy it to the heap for this to be safe
	heapSlice := make([]byte, len(slice))
	copy(heapSlice, slice)
	return heapSlice
}

// id returns a hash value based on the data and the last ID on the stack.
func (c *Context) id(data []byte) ID {
	const (
		// hashInitial is the initial value for the FNV-1a hash.
		// https://en.wikipedia.org/wiki/Fowler%E2%80%93Noll%E2%80%93Vo_hash_function
		hashInitial = 14695981039346656037
	)

	var init ID = hashInitial
	if len(c.idStack) > 0 {
		init = c.idStack[len(c.idStack)-1]
	}
	id := fnv1a(init, data)
	c.LastID = id
	return id
}

func (c *Context) pushID(data []byte) ID {
	// push()
	id := c.id(data)
	c.idStack = append(c.idStack, id)
	return id
}

func (c *Context) popID() {
	c.idStack = c.idStack[:len(c.idStack)-1]
}

func (c *Context) pushClipRect(rect image.Rectangle) {
	last := c.clipRect()
	// push()
	c.clipStack = append(c.clipStack, rect.Intersect(last))
}

func (c *Context) popClipRect() {
	c.clipStack = c.clipStack[:len(c.clipStack)-1]
}

func (c *Context) clipRect() image.Rectangle {
	return c.clipStack[len(c.clipStack)-1]
}

func (c *Context) checkClip(r image.Rectangle) int {
	cr := c.clipRect()
	if !r.Overlaps(cr) {
		return clipAll
	}
	if r.In(cr) {
		return 0
	}
	return clipPart
}

func (c *Context) layout() *layout {
	return &c.layoutStack[len(c.layoutStack)-1]
}

func (c *Context) popContainer() {
	cnt := c.CurrentContainer()
	layout := c.layout()
	cnt.ContentSize.X = layout.max.X - layout.body.Min.X
	cnt.ContentSize.Y = layout.max.Y - layout.body.Min.Y
	// pop container, layout and id
	// pop()
	c.containerStack = c.containerStack[:len(c.containerStack)-1]
	// pop()
	c.layoutStack = c.layoutStack[:len(c.layoutStack)-1]
}

func (c *Context) CurrentContainer() *Container {
	return c.containerStack[len(c.containerStack)-1]
}

func (c *Context) container(id ID, opt Option) *Container {
	// try to get existing container from pool
	if idx := c.poolGet(c.containerPool[:], id); idx >= 0 {
		if c.containers[idx].Open || (^opt&OptClosed) != 0 {
			c.poolUpdate(c.containerPool[:], idx)
		}
		return &c.containers[idx]
	}

	if (opt & OptClosed) != 0 {
		return nil
	}

	// container not found in pool: init new container
	idx := c.poolInit(c.containerPool[:], id)
	cnt := &c.containers[idx]
	*cnt = Container{}
	cnt.HeadIdx = -1
	cnt.TailIdx = -1
	cnt.Open = true
	c.bringToFront(cnt)
	return cnt
}

func (c *Context) Container(name string) *Container {
	id := c.id([]byte(name))
	return c.container(id, 0)
}

func (c *Context) bringToFront(cnt *Container) {
	c.lastZIndex++
	cnt.ZIndex = c.lastZIndex
}

func (c *Context) SetFocus(id ID) {
	c.focus = id
	c.keepFocus = true
}

func (c *Context) Update(f func()) {
	c.begin()
	defer c.end()
	f()
}

func (c *Context) begin() {
	c.updateInput()

	c.commandList = c.commandList[:0]
	c.rootList = c.rootList[:0]
	c.scrollTarget = nil
	c.hoverRoot = c.nextHoverRoot
	c.nextHoverRoot = nil
	c.mouseDelta.X = c.mousePos.X - c.lastMousePos.X
	c.mouseDelta.Y = c.mousePos.Y - c.lastMousePos.Y
	c.tick++
}

func (c *Context) end() {
	// check stacks
	expect(len(c.containerStack) == 0)
	expect(len(c.clipStack) == 0)
	expect(len(c.idStack) == 0)
	expect(len(c.layoutStack) == 0)

	// handle scroll input
	if c.scrollTarget != nil {
		c.scrollTarget.Scroll.X += c.scrollDelta.X
		c.scrollTarget.Scroll.Y += c.scrollDelta.Y
	}

	// unset focus if focus id was not touched this frame
	if !c.keepFocus {
		c.focus = 0
	}
	c.keepFocus = false

	// bring hover root to front if mouse was pressed
	if c.mousePressed != 0 && c.nextHoverRoot != nil &&
		c.nextHoverRoot.ZIndex < c.lastZIndex &&
		c.nextHoverRoot.ZIndex >= 0 {
		c.bringToFront(c.nextHoverRoot)
	}

	// reset input state
	c.keyPressed = 0
	c.textInput = nil
	c.mousePressed = 0
	c.scrollDelta = image.Pt(0, 0)
	c.lastMousePos = c.mousePos

	// sort root containers by zindex
	sort.SliceStable(c.rootList, func(i, j int) bool {
		return c.rootList[i].ZIndex < c.rootList[j].ZIndex
	})

	// set root container jump commands
	for i := 0; i < len(c.rootList); i++ {
		cnt := c.rootList[i]
		// if this is the first container then make the first command jump to it.
		// otherwise set the previous container's tail to jump to this one
		if i == 0 {
			cmd := c.commandList[0]
			expect(cmd.typ == commandJump)
			cmd.jump.dstIdx = cnt.HeadIdx + 1
			expect(cmd.jump.dstIdx < commandListSize)
		} else {
			prev := c.rootList[i-1]
			c.commandList[prev.TailIdx].jump.dstIdx = cnt.HeadIdx + 1
		}
		// make the last container's tail jump to the end of command list
		if i == len(c.rootList)-1 {
			c.commandList[cnt.TailIdx].jump.dstIdx = len(c.commandList)
		}
	}
}
