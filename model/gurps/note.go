// Copyright (c) 1998-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package gurps

import (
	"context"
	"io/fs"
	"strings"

	"github.com/richardwilkes/gcs/v5/model/gurps/enums/cell"
	"github.com/richardwilkes/gcs/v5/model/jio"
	"github.com/richardwilkes/gcs/v5/model/kinds"
	"github.com/richardwilkes/json"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/tid"
)

var (
	_ Node[*Note]       = &Note{}
	_ EditorData[*Note] = &NoteEditData{}
)

// Columns that can be used with the note method .CellData()
const (
	NoteTextColumn = iota
	NoteReferenceColumn
)

const (
	noteListTypeKey = "note_list"
	noteTypeKey     = "note"
)

// Note holds a note.
type Note struct {
	NoteData
	Entity *Entity
}

// NoteData holds the Note data that is written to disk.
type NoteData struct {
	TID tid.TID `json:"id"`
	NoteEditData
	ThirdParty map[string]any `json:"third_party,omitempty"`
	Children   []*Note        `json:"children,omitempty"` // Only for containers
	IsOpen     bool           `json:"open,omitempty"`     // Only for containers
	parent     *Note
}

// NoteEditData holds the Note data that can be edited by the UI detail editor.
type NoteEditData struct {
	Text             string `json:"text,omitempty"`
	PageRef          string `json:"reference,omitempty"`
	PageRefHighlight string `json:"reference_highlight,omitempty"`
}

type noteListData struct {
	Type    string  `json:"type"`
	Version int     `json:"version"`
	Rows    []*Note `json:"rows"`
}

// NewNotesFromFile loads an Note list from a file.
func NewNotesFromFile(fileSystem fs.FS, filePath string) ([]*Note, error) {
	var data noteListData
	if err := jio.LoadFromFS(context.Background(), fileSystem, filePath, &data); err != nil {
		return nil, errs.NewWithCause(invalidFileDataMsg(), err)
	}
	if data.Type != noteListTypeKey {
		return nil, errs.New(unexpectedFileDataMsg())
	}
	if err := CheckVersion(data.Version); err != nil {
		return nil, err
	}
	return data.Rows, nil
}

// SaveNotes writes the Note list to the file as JSON.
func SaveNotes(notes []*Note, filePath string) error {
	return jio.SaveToFile(context.Background(), filePath, &noteListData{
		Type:    noteListTypeKey,
		Version: CurrentDataVersion,
		Rows:    notes,
	})
}

// NewNote creates a new Note.
func NewNote(entity *Entity, parent *Note, container bool) *Note {
	n := Note{
		NoteData: NoteData{
			TID:    tid.MustNewTID(noteKind(container)),
			IsOpen: container,
			parent: parent,
		},
		Entity: entity,
	}
	n.Text = n.Kind()
	return &n
}

func noteKind(container bool) byte {
	if container {
		return kinds.NoteContainer
	}
	return kinds.Note
}

// ID returns the local ID of this data.
func (n *Note) ID() tid.TID {
	return n.TID
}

// Container returns true if this is a container.
func (n *Note) Container() bool {
	return tid.IsKind(n.TID, kinds.NoteContainer)
}

// HasChildren returns true if this node has children.
func (n *Note) HasChildren() bool {
	return n.Container() && len(n.Children) > 0
}

// NodeChildren returns the children of this node, if any.
func (n *Note) NodeChildren() []*Note {
	return n.Children
}

// SetChildren sets the children of this node.
func (n *Note) SetChildren(children []*Note) {
	n.Children = children
}

// Parent returns the parent.
func (n *Note) Parent() *Note {
	return n.parent
}

// SetParent sets the parent.
func (n *Note) SetParent(parent *Note) {
	n.parent = parent
}

// Open returns true if this node is currently open.
func (n *Note) Open() bool {
	return n.IsOpen && n.Container()
}

// SetOpen sets the current open state for this node.
func (n *Note) SetOpen(open bool) {
	n.IsOpen = open && n.Container()
}

// Clone implements Node.
func (n *Note) Clone(entity *Entity, parent *Note, preserveID bool) *Note {
	other := NewNote(entity, parent, n.Container())
	if preserveID {
		other.TID = n.TID
	}
	other.IsOpen = n.IsOpen
	other.ThirdParty = n.ThirdParty
	other.NoteEditData.CopyFrom(n)
	if n.HasChildren() {
		other.Children = make([]*Note, 0, len(n.Children))
		for _, child := range n.Children {
			other.Children = append(other.Children, child.Clone(entity, other, preserveID))
		}
	}
	return other
}

// MarshalJSON implements json.Marshaler.
func (n *Note) MarshalJSON() ([]byte, error) {
	type calc struct {
		ResolvedNotes string `json:"resolved_text,omitempty"`
	}
	n.ClearUnusedFieldsForType()
	data := struct {
		NoteData
		Calc *calc `json:"calc,omitempty"`
	}{
		NoteData: n.NoteData,
	}
	notes := n.resolveText()
	if notes != n.Text {
		data.Calc = &calc{ResolvedNotes: notes}
	}
	return json.Marshal(&data)
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *Note) UnmarshalJSON(data []byte) error {
	var localData struct {
		NoteData
		// Old data fields
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &localData); err != nil {
		return err
	}
	if !tid.IsValid(localData.TID) {
		// Fixup old data that used UUIDs instead of TIDs
		localData.TID = tid.MustNewTID(noteKind(strings.HasSuffix(localData.Type, ContainerKeyPostfix)))
	}
	n.NoteData = localData.NoteData
	n.ClearUnusedFieldsForType()
	if n.Container() {
		for _, one := range n.Children {
			one.parent = n
		}
	}
	return nil
}

func (n *Note) String() string {
	return n.resolveText()
}

func (n *Note) resolveText() string {
	return EvalEmbeddedRegex.ReplaceAllStringFunc(n.Text, n.Entity.EmbeddedEval)
}

// NotesHeaderData returns the header data information for the given note column.
func NotesHeaderData(columnID int) HeaderData {
	var data HeaderData
	switch columnID {
	case NoteTextColumn:
		data.Title = i18n.Text("Note")
		data.Primary = true
	case NoteReferenceColumn:
		data.Title = HeaderBookmark
		data.TitleIsImageKey = true
		data.Detail = PageRefTooltipText()
	}
	return data
}

// CellData returns the cell data information for the given column.
func (n *Note) CellData(columnID int, data *CellData) {
	switch columnID {
	case NoteTextColumn:
		data.Type = cell.Markdown
		data.Primary = n.resolveText()
	case NoteReferenceColumn, PageRefCellAlias:
		data.Type = cell.PageRef
		data.Primary = n.PageRef
		if n.PageRefHighlight != "" {
			data.Secondary = n.PageRefHighlight
		} else {
			data.Secondary = n.resolveText()
		}
	}
}

// Depth returns the number of parents this node has.
func (n *Note) Depth() int {
	count := 0
	p := n.parent
	for p != nil {
		count++
		p = p.parent
	}
	return count
}

// OwningEntity returns the owning Entity.
func (n *Note) OwningEntity() *Entity {
	return n.Entity
}

// SetOwningEntity sets the owning entity and configures any sub-components as needed.
func (n *Note) SetOwningEntity(entity *Entity) {
	n.Entity = entity
	if n.Container() {
		for _, child := range n.Children {
			child.SetOwningEntity(entity)
		}
	}
}

// Enabled returns true if this node is enabled.
func (n *Note) Enabled() bool {
	return true
}

// FillWithNameableKeys adds any nameable keys found to the provided map.
func (n *Note) FillWithNameableKeys(m map[string]string) {
	Extract(n.Text, m)
}

// ApplyNameableKeys replaces any nameable keys found with the corresponding values in the provided map.
func (n *Note) ApplyNameableKeys(m map[string]string) {
	n.Text = Apply(n.Text, m)
}

// CanConvertToFromContainer returns true if this node can be converted to/from a container.
func (n *Note) CanConvertToFromContainer() bool {
	return !n.Container() || !n.HasChildren()
}

// ConvertToContainer converts this node to a container.
func (n *Note) ConvertToContainer() {
	n.TID = tid.TID(kinds.NoteContainer) + n.TID[1:]
}

// ConvertToNonContainer converts this node to a non-container.
func (n *Note) ConvertToNonContainer() {
	n.TID = tid.TID(kinds.Note) + n.TID[1:]
}

// Kind returns the kind of data.
func (n *Note) Kind() string {
	if n.Container() {
		return i18n.Text("Note Container")
	}
	return i18n.Text("Note")
}

// ClearUnusedFieldsForType zeroes out the fields that are not applicable to this type (container vs not-container).
func (n *Note) ClearUnusedFieldsForType() {
	if !n.Container() {
		n.Children = nil
		n.IsOpen = false
	}
}

// CopyFrom implements node.EditorData.
func (n *NoteEditData) CopyFrom(other *Note) {
	n.copyFrom(&other.NoteEditData)
}

// ApplyTo implements node.EditorData.
func (n *NoteEditData) ApplyTo(other *Note) {
	other.NoteEditData.copyFrom(n)
}

func (n *NoteEditData) copyFrom(other *NoteEditData) {
	*n = *other
}
