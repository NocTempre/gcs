/*
 * Copyright ©1998-2023 by Richard A. Wilkes. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, version 2.0. If a copy of the MPL was not distributed with
 * this file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as
 * defined by the Mozilla Public License, version 2.0.
 */

package ux

import (
	"github.com/richardwilkes/unison"
)

var _ unison.Layout = &portraitLayout{}

type portraitLayout struct {
	portrait *PortraitPanel
	rest     *unison.Panel
}

func (p *portraitLayout) LayoutSizes(_ *unison.Panel, _ unison.Size) (min, pref, max unison.Size) {
	_, pref, _ = p.rest.Sizes(unison.Size{})
	insets := p.portrait.Border().Insets()
	pref.Width += 1 + pref.Height + insets.Width() - insets.Height()
	return pref, pref, pref
}

func (p *portraitLayout) PerformLayout(target *unison.Panel) {
	r := target.ContentRect(false)
	insets := p.portrait.Border().Insets()
	frame := r
	frame.Width = r.Height + insets.Width() - insets.Height()
	p.portrait.SetFrameRect(frame)
	r.X += frame.Width + 1
	r.Width -= frame.Width + 1
	p.rest.SetFrameRect(r)
}
