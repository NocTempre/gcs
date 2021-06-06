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

package com.trollworks.gcs.attribute;

import com.trollworks.gcs.utility.I18n;

public enum AttributeType {
    INTEGER {
        @Override
        public String toString() {
            return I18n.text("Integer");
        }
    },
    DECIMAL {
        @Override
        public String toString() {
            return I18n.text("Decimal");
        }
    },
    POOL {
        @Override
        public String toString() {
            return I18n.text("Pool");
        }
    }
}
