/*
 * Copyright ©1998-2021 by Richard A. Wilkes. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, version 2.0. If a copy of the MPL was not distributed with
 * this file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as
 * defined by the Mozilla Public License, version 2.0.
 */

package com.trollworks.gcs.character.panels;

import com.trollworks.gcs.character.CharacterSheet;
import com.trollworks.gcs.character.Encumbrance;
import com.trollworks.gcs.character.FieldFactory;
import com.trollworks.gcs.character.GURPSCharacter;
import com.trollworks.gcs.page.DropPanel;
import com.trollworks.gcs.page.PageField;
import com.trollworks.gcs.page.PageHeader;
import com.trollworks.gcs.page.PageLabel;
import com.trollworks.gcs.page.SpecialFontPageLabel;
import com.trollworks.gcs.ui.ThemeColor;
import com.trollworks.gcs.ui.layout.PrecisionLayout;
import com.trollworks.gcs.ui.layout.PrecisionLayoutAlignment;
import com.trollworks.gcs.ui.layout.PrecisionLayoutData;
import com.trollworks.gcs.ui.widget.Wrapper;
import com.trollworks.gcs.utility.I18n;
import com.trollworks.gcs.utility.text.Numbers;

import java.awt.Color;
import java.awt.Container;
import java.text.MessageFormat;
import javax.swing.SwingConstants;

/** The character encumbrance panel. */
public class EncumbrancePanel extends DropPanel {
    /**
     * Creates a new encumbrance panel.
     *
     * @param sheet The sheet to display the data for.
     */
    public EncumbrancePanel(CharacterSheet sheet) {
        super(new PrecisionLayout().setColumns(8).setHorizontalSpacing(2).setVerticalSpacing(0).setMargins(0), I18n.text("Encumbrance, Move & Dodge"), true);

        PageHeader header = new PageHeader(I18n.text("Level"), I18n.text("The encumbrance level"));
        add(header, new PrecisionLayoutData().setHorizontalSpan(2).setHorizontalAlignment(PrecisionLayoutAlignment.MIDDLE).setGrabHorizontalSpace(true));
        addHorizontalBackground(header, ThemeColor.HEADER);

        addVerticalBackground(createDivider(), ThemeColor.DIVIDER);

        String maxLoadTooltip = I18n.text("The maximum load a character can carry and still remain within a specific encumbrance level");
        header = new PageHeader(I18n.text("Max Load"), maxLoadTooltip);
        add(header, new PrecisionLayoutData().setHorizontalAlignment(PrecisionLayoutAlignment.MIDDLE));

        addVerticalBackground(createDivider(), ThemeColor.DIVIDER);

        String moveTooltip = I18n.text("The character's ground movement rate for a specific encumbrance level");
        header = new PageHeader(I18n.text("Move"), moveTooltip);
        add(header, new PrecisionLayoutData().setHorizontalAlignment(PrecisionLayoutAlignment.MIDDLE));

        addVerticalBackground(createDivider(), ThemeColor.DIVIDER);

        String dodgeTooltip = I18n.text("The character's dodge for a specific encumbrance level");
        header = new PageHeader(I18n.text("Dodge"), dodgeTooltip);
        add(header, new PrecisionLayoutData().setHorizontalAlignment(PrecisionLayoutAlignment.MIDDLE));

        GURPSCharacter character = sheet.getCharacter();
        Encumbrance    current   = character.getEncumbranceLevel(false);
        boolean        band      = false;
        for (Encumbrance encumbrance : Encumbrance.values()) {
            boolean warn;
            Color   textColor;
            if (current == encumbrance) {
                warn = character.isCarryingGreaterThanMaxLoad(false);
                textColor = warn ? ThemeColor.ON_WARNING : ThemeColor.ON_CURRENT;
            } else {
                warn = false;
                textColor = ThemeColor.ON_CONTENT;
            }

            if (encumbrance == current) {
                add(new SpecialFontPageLabel("\uf24e", textColor, header), new PrecisionLayoutData().setFillHorizontalAlignment());
            } else {
                add(new PageLabel(" ", textColor, header), new PrecisionLayoutData().setFillHorizontalAlignment());
            }

            PageLabel level = new PageLabel(MessageFormat.format("{0} {1}", Numbers.format(-encumbrance.getEncumbrancePenalty()), encumbrance), textColor, header);
            add(level, new PrecisionLayoutData().setGrabHorizontalSpace(true));
            if (current == encumbrance) {
                addHorizontalBackground(level, warn ? ThemeColor.WARNING : ThemeColor.MARKER);
            } else if (band) {
                addHorizontalBackground(level, ThemeColor.BANDING);
            }
            band = !band;
            createDivider();
            add(new PageField(FieldFactory.WEIGHT, character.getMaximumCarry(encumbrance), sheet, SwingConstants.RIGHT, maxLoadTooltip, textColor), new PrecisionLayoutData().setFillHorizontalAlignment());
            createDivider();
            add(new PageField(FieldFactory.POSINT5, Integer.valueOf(character.getMove(encumbrance)), sheet, SwingConstants.RIGHT, moveTooltip, textColor), new PrecisionLayoutData().setFillHorizontalAlignment());
            createDivider();
            add(new PageField(FieldFactory.POSINT5, Integer.valueOf(character.getDodge(encumbrance)), sheet, SwingConstants.RIGHT, dodgeTooltip, textColor), new PrecisionLayoutData().setFillHorizontalAlignment());
        }
    }

    private Container createDivider() {
        Wrapper panel = new Wrapper();
        panel.setOnlySize(1, 1);
        add(panel);
        return panel;
    }
}
