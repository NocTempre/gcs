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
	"slices"
	"strings"

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/affects"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/cell"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/display"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/tmcost"
	"github.com/richardwilkes/gcs/v5/model/jio"
	"github.com/richardwilkes/json"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/tid"
	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/unison/enums/align"
)

var (
	_ Node[*TraitModifier]       = &TraitModifier{}
	_ GeneralModifier            = &TraitModifier{}
	_ LeveledOwner               = &TraitModifier{}
	_ EditorData[*TraitModifier] = &TraitModifier{}
)

// Columns that can be used with the trait modifier method .CellData()
const (
	TraitModifierEnabledColumn = iota
	TraitModifierDescriptionColumn
	TraitModifierCostColumn
	TraitModifierTagsColumn
	TraitModifierReferenceColumn
)

const (
	traitModifierListTypeKey = "modifier_list"
	traitModifierTypeKey     = "modifier"
)

// GeneralModifier is used for common access to modifiers.
type GeneralModifier interface {
	Container() bool
	Depth() int
	FullDescription() string
	FullCostDescription() string
	Enabled() bool
	SetEnabled(enabled bool)
}

// TraitModifier holds a modifier to an Trait.
type TraitModifier struct {
	TraitModifierData
	Entity *Entity
}

// TraitModifierData holds the TraitModifier data that is written to disk.
type TraitModifierData struct {
	ContainerBase[*TraitModifier]
	TraitModifierEditData
}

// TraitModifierEditData holds the TraitModifier data that can be edited by the UI detail editor.
type TraitModifierEditData struct {
	Name             string   `json:"name,omitempty"`
	PageRef          string   `json:"reference,omitempty"`
	PageRefHighlight string   `json:"reference_highlight,omitempty"`
	LocalNotes       string   `json:"notes,omitempty"`
	VTTNotes         string   `json:"vtt_notes,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	TraitModifierEditDataNonContainerOnly
}

// TraitModifierEditDataNonContainerOnly holds the TraitModifier data that is only applicable to
// TraitModifiers that aren't containers.
type TraitModifierEditDataNonContainerOnly struct {
	Cost     fxp.Int        `json:"cost,omitempty"`
	Levels   fxp.Int        `json:"levels,omitempty"`
	Affects  affects.Option `json:"affects,omitempty"`
	CostType tmcost.Type    `json:"cost_type,omitempty"`
	Disabled bool           `json:"disabled,omitempty"`
	Features Features       `json:"features,omitempty"`
}

type traitModifierListData struct {
	Type    string           `json:"type"`
	Version int              `json:"version"`
	Rows    []*TraitModifier `json:"rows"`
}

// NewTraitModifiersFromFile loads a TraitModifier list from a file.
func NewTraitModifiersFromFile(fileSystem fs.FS, filePath string) ([]*TraitModifier, error) {
	var data traitModifierListData
	if err := jio.LoadFromFS(context.Background(), fileSystem, filePath, &data); err != nil {
		return nil, errs.NewWithCause(InvalidFileDataMsg(), err)
	}
	if data.Type != traitModifierListTypeKey {
		return nil, errs.New(UnexpectedFileDataMsg())
	}
	if err := CheckVersion(data.Version); err != nil {
		return nil, err
	}
	return data.Rows, nil
}

// SaveTraitModifiers writes the TraitModifier list to the file as JSON.
func SaveTraitModifiers(modifiers []*TraitModifier, filePath string) error {
	return jio.SaveToFile(context.Background(), filePath, &traitModifierListData{
		Type:    traitModifierListTypeKey,
		Version: CurrentDataVersion,
		Rows:    modifiers,
	})
}

// NewTraitModifier creates a TraitModifier.
func NewTraitModifier(entity *Entity, parent *TraitModifier, container bool) *TraitModifier {
	m := TraitModifier{
		TraitModifierData: TraitModifierData{
			ContainerBase: newContainerBase(parent, KindTraitModifier, KindTraitModifierContainer, container),
		},
		Entity: entity,
	}
	m.Name = m.Kind()
	return &m
}

// Clone implements Node.
func (m *TraitModifier) Clone(entity *Entity, parent *TraitModifier, preserveID bool) *TraitModifier {
	other := NewTraitModifier(entity, parent, m.Container())
	other.CopyFrom(m)
	if preserveID {
		other.LocalID = m.LocalID
	}
	if m.HasChildren() {
		other.Children = make([]*TraitModifier, 0, len(m.Children))
		for _, child := range m.Children {
			other.Children = append(other.Children, child.Clone(entity, other, preserveID))
		}
	}
	return other
}

// MarshalJSON implements json.Marshaler.
func (m *TraitModifier) MarshalJSON() ([]byte, error) {
	m.ClearUnusedFieldsForType()
	return json.Marshal(&m.TraitModifierData)
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *TraitModifier) UnmarshalJSON(data []byte) error {
	var localData struct {
		TraitModifierData
		// Old data fields
		Type       string   `json:"type"`
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(data, &localData); err != nil {
		return err
	}
	localData.itemKind = KindTraitModifier
	localData.containerKind = KindTraitModifierContainer
	if !tid.IsKindAndValid(localData.LocalID, KindTraitModifierContainer) && !tid.IsKindAndValid(localData.LocalID, KindTraitModifier) {
		switch localData.Type {
		case "modifier":
			localData.LocalID = tid.MustNewTID(KindTraitModifier)
		case "modifier_container":
			localData.LocalID = tid.MustNewTID(KindTraitModifierContainer)
		default:
			return errs.New("invalid data type")
		}
	}
	m.TraitModifierData = localData.TraitModifierData
	m.ClearUnusedFieldsForType()
	m.Tags = ConvertOldCategoriesToTags(m.Tags, localData.Categories)
	slices.Sort(m.Tags)
	if m.Container() {
		for _, one := range m.Children {
			one.parent = m
		}
	}
	return nil
}

// TagList returns the list of tags.
func (m *TraitModifier) TagList() []string {
	return m.Tags
}

// CellData returns the cell data information for the given column.
func (m *TraitModifier) CellData(columnID int, data *CellData) {
	switch columnID {
	case TraitModifierEnabledColumn:
		if !m.Container() {
			data.Type = cell.Toggle
			data.Checked = m.Enabled()
			data.Alignment = align.Middle
		}
	case TraitModifierDescriptionColumn:
		data.Type = cell.Text
		data.Primary = m.Name
		data.Secondary = m.SecondaryText(func(option display.Option) bool { return option.Inline() })
		data.Tooltip = m.SecondaryText(func(option display.Option) bool { return option.Tooltip() })
	case TraitModifierCostColumn:
		if !m.Container() {
			data.Type = cell.Text
			data.Primary = m.CostDescription()
		}
	case TraitModifierTagsColumn:
		data.Type = cell.Tags
		data.Primary = CombineTags(m.Tags)
	case TraitModifierReferenceColumn, PageRefCellAlias:
		data.Type = cell.PageRef
		data.Primary = m.PageRef
		if m.PageRefHighlight != "" {
			data.Secondary = m.PageRefHighlight
		} else {
			data.Secondary = m.Name
		}
	}
}

// Depth returns the number of parents this node has.
func (m *TraitModifier) Depth() int {
	count := 0
	p := m.parent
	for p != nil {
		count++
		p = p.parent
	}
	return count
}

// OwningEntity returns the owning Entity.
func (m *TraitModifier) OwningEntity() *Entity {
	return m.Entity
}

// SetOwningEntity sets the owning entity and configures any sub-components as needed.
func (m *TraitModifier) SetOwningEntity(entity *Entity) {
	m.Entity = entity
	if m.Container() {
		for _, child := range m.Children {
			child.SetOwningEntity(entity)
		}
	}
}

// CostModifier returns the total cost modifier.
func (m *TraitModifier) CostModifier() fxp.Int {
	if m.Levels > 0 {
		return m.Cost.Mul(m.Levels)
	}
	return m.Cost
}

// IsLeveled returns true if this TraitModifier is leveled.
func (m *TraitModifier) IsLeveled() bool {
	return !m.Container() && m.CostType == tmcost.Percentage && m.Levels > 0
}

// CurrentLevel returns the current level of the modifier or zero if it is not leveled.
func (m *TraitModifier) CurrentLevel() fxp.Int {
	if m.Enabled() && m.IsLeveled() {
		return m.Levels
	}
	return 0
}

func (m *TraitModifier) String() string {
	var buffer strings.Builder
	buffer.WriteString(m.Name)
	if m.IsLeveled() {
		buffer.WriteByte(' ')
		buffer.WriteString(m.Levels.String())
	}
	return buffer.String()
}

// SecondaryText returns the "secondary" text: the text display below an Trait.
func (m *TraitModifier) SecondaryText(optionChecker func(display.Option) bool) string {
	if optionChecker(SheetSettingsFor(m.Entity).NotesDisplay) {
		return m.LocalNotes
	}
	return ""
}

// FullDescription returns a full description.
func (m *TraitModifier) FullDescription() string {
	var buffer strings.Builder
	buffer.WriteString(m.String())
	if m.LocalNotes != "" {
		buffer.WriteString(" (")
		buffer.WriteString(m.LocalNotes)
		buffer.WriteByte(')')
	}
	if SheetSettingsFor(m.Entity).ShowTraitModifierAdj {
		buffer.WriteString(" [")
		buffer.WriteString(m.CostDescription())
		buffer.WriteByte(']')
	}
	return buffer.String()
}

// FullCostDescription is the same as CostDescription().
func (m *TraitModifier) FullCostDescription() string {
	return m.CostDescription()
}

// CostDescription returns the formatted cost.
func (m *TraitModifier) CostDescription() string {
	if m.Container() {
		return ""
	}
	var base string
	switch m.CostType {
	case tmcost.Percentage:
		if m.IsLeveled() {
			base = m.Cost.Mul(m.Levels).StringWithSign()
		} else {
			base = m.Cost.StringWithSign()
		}
		base += tmcost.Percentage.String()
	case tmcost.Points:
		base = m.Cost.StringWithSign()
	case tmcost.Multiplier:
		return m.CostType.String() + m.Cost.String()
	default:
		errs.Log(errs.New("unknown cost type"), "type", int(m.CostType))
		base = m.Cost.StringWithSign() + tmcost.Percentage.String()
	}
	if desc := m.Affects.AltString(); desc != "" {
		base += " " + desc
	}
	return base
}

// FillWithNameableKeys adds any nameable keys found in this TraitModifier to the provided map.
func (m *TraitModifier) FillWithNameableKeys(keyMap map[string]string) {
	if !m.Container() && m.Enabled() {
		Extract(m.Name, keyMap)
		Extract(m.LocalNotes, keyMap)
		for _, one := range m.Features {
			one.FillWithNameableKeys(keyMap)
		}
	}
}

// ApplyNameableKeys replaces any nameable keys found in this TraitModifier with the corresponding values in the provided map.
func (m *TraitModifier) ApplyNameableKeys(keyMap map[string]string) {
	if !m.Container() && m.Enabled() {
		m.Name = Apply(m.Name, keyMap)
		m.LocalNotes = Apply(m.LocalNotes, keyMap)
		for _, one := range m.Features {
			one.ApplyNameableKeys(keyMap)
		}
	}
}

// Enabled returns true if this node is enabled.
func (m *TraitModifier) Enabled() bool {
	return !m.Disabled || m.Container()
}

// SetEnabled makes the node enabled, if possible.
func (m *TraitModifier) SetEnabled(enabled bool) {
	if !m.Container() {
		m.Disabled = !enabled
	}
}

// Kind returns the kind of data.
func (m *TraitModifier) Kind() string {
	return m.kind(i18n.Text("Trait Modifier"))
}

// ClearUnusedFieldsForType zeroes out the fields that are not applicable to this type (container vs not-container).
func (m *TraitModifier) ClearUnusedFieldsForType() {
	m.clearUnusedFields()
	if m.Container() {
		m.CostType = 0
		m.Disabled = false
		m.Cost = 0
		m.Levels = 0
		m.Affects = 0
		m.Features = nil
	}
}

// CopyFrom implements node.EditorData.
func (m *TraitModifier) CopyFrom(other *TraitModifier) {
	m.copyFrom(other)
	m.LocalID = tid.MustNewTID(m.LocalID[0])
}

// ApplyTo implements node.EditorData.
func (m *TraitModifier) ApplyTo(other *TraitModifier) {
	id := other.LocalID
	other.copyFrom(m)
	other.LocalID = id
}

func (m *TraitModifier) copyFrom(other *TraitModifier) {
	m.TraitModifierData = other.TraitModifierData
	m.Tags = txt.CloneStringSlice(m.Tags)
	m.Features = other.Features.Clone()
}
