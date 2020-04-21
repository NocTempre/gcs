/*
 * Copyright (c) 1998-2020 by Richard A. Wilkes. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, version 2.0.
 * If a copy of the MPL was not distributed with this file, You can obtain one at
 * http://mozilla.org/MPL/2.0/.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the
 * Mozilla Public License, version 2.0.
 */

package com.trollworks.gcs.menu.edit;

/** Objects that can have their uses incremented/decremented should implement this interface. */
public interface UsesIncrementable {
    /** @return Whether the uses can be incremented. */
    boolean canIncrementUses();

    /** @return Whether the uses can be decremented. */
    boolean canDecrementUses();

    /** Call to increment the uses. */
    void incrementUses();

    /** Call to decrement the uses. */
    void decrementUses();
}
