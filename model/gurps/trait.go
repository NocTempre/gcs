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
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/container"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/display"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/selfctrl"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/study"
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
	_ WeaponOwner            = &Trait{}
	_ Node[*Trait]           = &Trait{}
	_ TemplatePickerProvider = &Trait{}
	_ LeveledOwner           = &Trait{}
	_ EditorData[*Trait]     = &Trait{}
)

// Columns that can be used with the trait method .CellData()
const (
	TraitDescriptionColumn = iota
	TraitPointsColumn
	TraitTagsColumn
	TraitReferenceColumn
)

const (
	traitListTypeKey = "trait_list"
	traitTypeKey     = "trait"
)

// Trait holds an advantage, disadvantage, quirk, or perk.
type Trait struct {
	TraitData
	Entity            *Entity
	UnsatisfiedReason string
}

// TraitData holds the Trait data that is written to disk.
type TraitData struct {
	ContainerBase[*Trait]
	TraitEditData
}

// TraitEditData holds the Trait data that can be edited by the UI detail editor.
type TraitEditData struct {
	Name             string              `json:"name,omitempty"`
	PageRef          string              `json:"reference,omitempty"`
	PageRefHighlight string              `json:"reference_highlight,omitempty"`
	LocalNotes       string              `json:"notes,omitempty"`
	VTTNotes         string              `json:"vtt_notes,omitempty"`
	UserDesc         string              `json:"userdesc,omitempty"`
	Tags             []string            `json:"tags,omitempty"`
	Modifiers        []*TraitModifier    `json:"modifiers,omitempty"`
	CR               selfctrl.Roll       `json:"cr,omitempty"`
	CRAdj            selfctrl.Adjustment `json:"cr_adj,omitempty"`
	Disabled         bool                `json:"disabled,omitempty"`
	TraitNonContainerOnlyEditData
	TraitContainerOnlyEditData
}

// TraitNonContainerOnlyEditData holds the Trait data that is only applicable to traits that aren't containers.
type TraitNonContainerOnlyEditData struct {
	BasePoints       fxp.Int     `json:"base_points,omitempty"`
	Levels           fxp.Int     `json:"levels,omitempty"`
	PointsPerLevel   fxp.Int     `json:"points_per_level,omitempty"`
	Prereq           *PrereqList `json:"prereqs,omitempty"`
	Weapons          []*Weapon   `json:"weapons,omitempty"`
	Features         Features    `json:"features,omitempty"`
	Study            []*Study    `json:"study,omitempty"`
	StudyHoursNeeded study.Level `json:"study_hours_needed,omitempty"`
	RoundCostDown    bool        `json:"round_down,omitempty"`
	CanLevel         bool        `json:"can_level,omitempty"`
}

// TraitContainerOnlyEditData holds the Trait data that is only applicable to traits that are containers.
type TraitContainerOnlyEditData struct {
	Ancestry       string          `json:"ancestry,omitempty"`
	TemplatePicker *TemplatePicker `json:"template_picker,omitempty"`
	ContainerType  container.Type  `json:"container_type,omitempty"`
}

type traitListData struct {
	Type    string   `json:"type"`
	Version int      `json:"version"`
	Rows    []*Trait `json:"rows"`
}

// NewTraitsFromFile loads an Trait list from a file.
func NewTraitsFromFile(fileSystem fs.FS, filePath string) ([]*Trait, error) {
	var data traitListData
	if err := jio.LoadFromFS(context.Background(), fileSystem, filePath, &data); err != nil {
		return nil, errs.NewWithCause(InvalidFileDataMsg(), err)
	}
	if data.Type == "advantage_list" {
		data.Type = traitListTypeKey
	}
	if data.Type != traitListTypeKey {
		return nil, errs.New(UnexpectedFileDataMsg())
	}
	if err := CheckVersion(data.Version); err != nil {
		return nil, err
	}
	return data.Rows, nil
}

// SaveTraits writes the Trait list to the file as JSON.
func SaveTraits(traits []*Trait, filePath string) error {
	return jio.SaveToFile(context.Background(), filePath, &traitListData{
		Type:    traitListTypeKey,
		Version: CurrentDataVersion,
		Rows:    traits,
	})
}

// NewTrait creates a new Trait.
func NewTrait(entity *Entity, parent *Trait, container bool) *Trait {
	t := Trait{
		TraitData: TraitData{
			ContainerBase: newContainerBase(parent, KindTrait, KindTraitContainer, container),
		},
		Entity: entity,
	}
	t.Name = t.Kind()
	if t.Container() {
		t.TemplatePicker = &TemplatePicker{}
	}
	return &t
}

// Clone implements Node.
func (t *Trait) Clone(entity *Entity, parent *Trait, preserveID bool) *Trait {
	other := NewTrait(entity, parent, t.Container())
	other.CopyFrom(t)
	if preserveID {
		other.LocalID = t.LocalID
	}
	if t.HasChildren() {
		other.Children = make([]*Trait, 0, len(t.Children))
		for _, child := range t.Children {
			other.Children = append(other.Children, child.Clone(entity, other, preserveID))
		}
	}
	return other
}

// MarshalJSON implements json.Marshaler.
func (t *Trait) MarshalJSON() ([]byte, error) {
	type calc struct {
		Points            fxp.Int `json:"points"`
		UnsatisfiedReason string  `json:"unsatisfied_reason,omitempty"`
		ResolvedNotes     string  `json:"resolved_notes,omitempty"`
	}
	t.ClearUnusedFieldsForType()
	data := struct {
		TraitData
		Calc calc `json:"calc"`
	}{
		TraitData: t.TraitData,
		Calc: calc{
			Points:            t.AdjustedPoints(),
			UnsatisfiedReason: t.UnsatisfiedReason,
		},
	}
	notes := t.resolveLocalNotes()
	if notes != t.LocalNotes {
		data.Calc.ResolvedNotes = notes
	}
	return json.Marshal(&data)
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *Trait) UnmarshalJSON(data []byte) error {
	var localData struct {
		TraitData
		// Old data fields
		Type         string   `json:"type"`
		Categories   []string `json:"categories"`
		Mental       bool     `json:"mental"`
		Physical     bool     `json:"physical"`
		Social       bool     `json:"social"`
		Exotic       bool     `json:"exotic"`
		Supernatural bool     `json:"supernatural"`
	}
	if err := json.Unmarshal(data, &localData); err != nil {
		return err
	}

	// Swap out old type keys
	switch localData.Type {
	case "advantage":
		localData.Type = traitTypeKey
	case "advantage_container":
		localData.Type = traitTypeKey + ContainerKeyPostfix
	}

	localData.itemKind = KindEquipment
	localData.containerKind = KindEquipmentContainer
	if !tid.IsKindAndValid(localData.LocalID, KindTraitContainer) && !tid.IsKindAndValid(localData.LocalID, KindTrait) {
		switch localData.Type {
		case "trait":
			localData.LocalID = tid.MustNewTID(KindTrait)
		case "trait_container":
			localData.LocalID = tid.MustNewTID(KindTraitContainer)
		default:
			return errs.New("invalid data type")
		}
	}

	// Force the CanLevel flag, if needed
	if !localData.Container() && (localData.Levels != 0 || localData.PointsPerLevel != 0) {
		localData.CanLevel = true
	}
	t.TraitData = localData.TraitData

	t.ClearUnusedFieldsForType()
	t.transferOldTypeFlagToTags(i18n.Text("Mental"), localData.Mental)
	t.transferOldTypeFlagToTags(i18n.Text("Physical"), localData.Physical)
	t.transferOldTypeFlagToTags(i18n.Text("Social"), localData.Social)
	t.transferOldTypeFlagToTags(i18n.Text("Exotic"), localData.Exotic)
	t.transferOldTypeFlagToTags(i18n.Text("Supernatural"), localData.Supernatural)
	t.Tags = ConvertOldCategoriesToTags(t.Tags, localData.Categories)
	slices.Sort(t.Tags)
	if t.Container() {
		for _, one := range t.Children {
			one.parent = t
		}
	}
	return nil
}

func (t *Trait) transferOldTypeFlagToTags(name string, flag bool) {
	if flag && !slices.Contains(t.Tags, name) {
		t.Tags = append(t.Tags, name)
	}
}

// EffectivelyDisabled returns true if this node or a parent is disabled.
func (t *Trait) EffectivelyDisabled() bool {
	if t.Disabled {
		return true
	}
	p := t.Parent()
	for p != nil {
		if p.Disabled {
			return true
		}
		p = p.Parent()
	}
	return false
}

// TemplatePickerData returns the TemplatePicker data, if any.
func (t *Trait) TemplatePickerData() *TemplatePicker {
	return t.TemplatePicker
}

// TraitsHeaderData returns the header data information for the given trait column.
func TraitsHeaderData(columnID int) HeaderData {
	var data HeaderData
	switch columnID {
	case TraitDescriptionColumn:
		data.Title = i18n.Text("Trait")
		data.Primary = true
	case TraitPointsColumn:
		data.Title = i18n.Text("Pts")
		data.Detail = i18n.Text("Points")
	case TraitTagsColumn:
		data.Title = i18n.Text("Tags")
	case TraitReferenceColumn:
		data.Title = HeaderBookmark
		data.TitleIsImageKey = true
		data.Detail = PageRefTooltipText()
	}
	return data
}

// CellData returns the cell data information for the given column.
func (t *Trait) CellData(columnID int, data *CellData) {
	data.Dim = !t.Enabled()
	switch columnID {
	case TraitDescriptionColumn:
		data.Type = cell.Text
		data.Primary = t.String()
		data.Secondary = t.SecondaryText(func(option display.Option) bool { return option.Inline() })
		data.Disabled = t.EffectivelyDisabled()
		data.UnsatisfiedReason = t.UnsatisfiedReason
		data.Tooltip = t.SecondaryText(func(option display.Option) bool { return option.Tooltip() })
		data.TemplateInfo = t.TemplatePicker.Description()
		if t.Container() {
			switch t.ContainerType {
			case container.AlternativeAbilities:
				data.InlineTag = i18n.Text("Alternate")
			case container.Ancestry:
				data.InlineTag = i18n.Text("Ancestry")
			case container.Attributes:
				data.InlineTag = i18n.Text("Attribute")
			case container.MetaTrait:
				data.InlineTag = i18n.Text("Meta")
			default:
			}
		}
	case TraitPointsColumn:
		data.Type = cell.Text
		data.Primary = t.AdjustedPoints().String()
		data.Alignment = align.End
	case TraitTagsColumn:
		data.Type = cell.Tags
		data.Primary = CombineTags(t.Tags)
	case TraitReferenceColumn, PageRefCellAlias:
		data.Type = cell.PageRef
		data.Primary = t.PageRef
		if t.PageRefHighlight != "" {
			data.Secondary = t.PageRefHighlight
		} else {
			data.Secondary = t.Name
		}
	}
}

// Depth returns the number of parents this node has.
func (t *Trait) Depth() int {
	count := 0
	p := t.parent
	for p != nil {
		count++
		p = p.parent
	}
	return count
}

// OwningEntity returns the owning Entity.
func (t *Trait) OwningEntity() *Entity {
	return t.Entity
}

// SetOwningEntity sets the owning entity and configures any sub-components as needed.
func (t *Trait) SetOwningEntity(entity *Entity) {
	t.Entity = entity
	if t.Container() {
		for _, child := range t.Children {
			child.SetOwningEntity(entity)
		}
	} else {
		for _, w := range t.Weapons {
			w.SetOwner(t)
		}
	}
	for _, m := range t.Modifiers {
		m.SetOwningEntity(entity)
	}
}

// Notes returns the local notes.
func (t *Trait) Notes() string {
	return t.resolveLocalNotes()
}

// IsLeveled returns true if the Trait is capable of having levels.
func (t *Trait) IsLeveled() bool {
	return t.CanLevel && !t.Container()
}

// CurrentLevel returns the current level of the trait or zero if it is not leveled.
func (t *Trait) CurrentLevel() fxp.Int {
	if t.Enabled() && t.IsLeveled() {
		return t.Levels
	}
	return 0
}

// AdjustedPoints returns the total points, taking levels and modifiers into account.
func (t *Trait) AdjustedPoints() fxp.Int {
	if t.EffectivelyDisabled() {
		return 0
	}
	if !t.Container() {
		return AdjustedPoints(t.Entity, t.CanLevel, t.BasePoints, t.Levels, t.PointsPerLevel, t.CR, t.AllModifiers(), t.RoundCostDown)
	}
	var points fxp.Int
	if t.ContainerType == container.AlternativeAbilities {
		values := make([]fxp.Int, len(t.Children))
		for i, one := range t.Children {
			values[i] = one.AdjustedPoints()
			if values[i] > points {
				points = values[i]
			}
		}
		maximum := points
		found := false
		for _, v := range values {
			if !found && maximum == v {
				found = true
			} else {
				points += fxp.ApplyRounding(calculateModifierPoints(v, fxp.Twenty), t.RoundCostDown)
			}
		}
	} else {
		for _, one := range t.Children {
			points += one.AdjustedPoints()
		}
	}
	return points
}

// AllModifiers returns the modifiers plus any inherited from parents.
func (t *Trait) AllModifiers() []*TraitModifier {
	all := make([]*TraitModifier, len(t.Modifiers))
	copy(all, t.Modifiers)
	p := t.parent
	for p != nil {
		all = append(all, p.Modifiers...)
		p = p.parent
	}
	return all
}

// Enabled returns true if this Trait and all of its parents are enabled.
func (t *Trait) Enabled() bool {
	if t.Disabled {
		return false
	}
	p := t.parent
	for p != nil {
		if p.Disabled {
			return false
		}
		p = p.parent
	}
	return true
}

// Description returns a description, which doesn't include any levels.
func (t *Trait) Description() string {
	return t.Name
}

// String implements fmt.Stringer.
func (t *Trait) String() string {
	var buffer strings.Builder
	buffer.WriteString(t.Name)
	if t.IsLeveled() {
		buffer.WriteByte(' ')
		buffer.WriteString(t.Levels.String())
	}
	return buffer.String()
}

func (t *Trait) resolveLocalNotes() string {
	return EvalEmbeddedRegex.ReplaceAllStringFunc(t.LocalNotes, t.Entity.EmbeddedEval)
}

// FeatureList returns the list of Features.
func (t *Trait) FeatureList() Features {
	return t.Features
}

// TagList returns the list of tags.
func (t *Trait) TagList() []string {
	return t.Tags
}

// RatedStrength always return 0 for traits.
func (t *Trait) RatedStrength() fxp.Int {
	return 0
}

// FillWithNameableKeys adds any nameable keys found to the provided map.
func (t *Trait) FillWithNameableKeys(m map[string]string) {
	Extract(t.Name, m)
	Extract(t.LocalNotes, m)
	Extract(t.UserDesc, m)
	if t.Prereq != nil {
		t.Prereq.FillWithNameableKeys(m)
	}
	for _, one := range t.Features {
		one.FillWithNameableKeys(m)
	}
	for _, one := range t.Weapons {
		one.FillWithNameableKeys(m)
	}
	Traverse(func(mod *TraitModifier) bool {
		mod.FillWithNameableKeys(m)
		return false
	}, true, true, t.Modifiers...)
}

// ApplyNameableKeys replaces any nameable keys found with the corresponding values in the provided map.
func (t *Trait) ApplyNameableKeys(m map[string]string) {
	t.Name = Apply(t.Name, m)
	t.LocalNotes = Apply(t.LocalNotes, m)
	t.UserDesc = Apply(t.UserDesc, m)
	if t.Prereq != nil {
		t.Prereq.ApplyNameableKeys(m)
	}
	for _, one := range t.Features {
		one.ApplyNameableKeys(m)
	}
	for _, one := range t.Weapons {
		one.ApplyNameableKeys(m)
	}
	Traverse(func(mod *TraitModifier) bool {
		mod.ApplyNameableKeys(m)
		return false
	}, true, true, t.Modifiers...)
}

// ActiveModifierFor returns the first modifier that matches the name (case-insensitive).
func (t *Trait) ActiveModifierFor(name string) *TraitModifier {
	var found *TraitModifier
	Traverse(func(mod *TraitModifier) bool {
		if strings.EqualFold(mod.Name, name) {
			found = mod
			return true
		}
		return false
	}, true, true, t.Modifiers...)
	return found
}

// ModifierNotes returns the notes due to modifiers.
func (t *Trait) ModifierNotes() string {
	var buffer strings.Builder
	if t.CR != selfctrl.NoCR {
		buffer.WriteString(t.CR.String())
		if t.CRAdj != selfctrl.NoCRAdj {
			buffer.WriteString(", ")
			buffer.WriteString(t.CRAdj.Description(t.CR))
		}
	}
	Traverse(func(mod *TraitModifier) bool {
		if buffer.Len() != 0 {
			buffer.WriteString("; ")
		}
		buffer.WriteString(mod.FullDescription())
		return false
	}, true, true, t.Modifiers...)
	return buffer.String()
}

// SecondaryText returns the "secondary" text: the text display below an Trait.
func (t *Trait) SecondaryText(optionChecker func(display.Option) bool) string {
	var buffer strings.Builder
	settings := SheetSettingsFor(t.Entity)
	if t.UserDesc != "" && optionChecker(settings.UserDescriptionDisplay) {
		buffer.WriteString(t.UserDesc)
	}
	if optionChecker(settings.ModifiersDisplay) {
		AppendStringOntoNewLine(&buffer, t.ModifierNotes())
	}
	if optionChecker(settings.NotesDisplay) {
		AppendStringOntoNewLine(&buffer, strings.TrimSpace(t.Notes()))
		AppendStringOntoNewLine(&buffer, StudyHoursProgressText(ResolveStudyHours(t.Study), t.StudyHoursNeeded, false))
	}
	return buffer.String()
}

// HasTag returns true if 'tag' is present in 'tags'. This check both ignores case and can check for subsets that are
// colon-separated.
func HasTag(tag string, tags []string) bool {
	tag = strings.TrimSpace(tag)
	for _, one := range tags {
		for _, part := range strings.Split(one, ":") {
			if strings.EqualFold(tag, strings.TrimSpace(part)) {
				return true
			}
		}
	}
	return false
}

// CombineTags combines multiple tags into a single string.
func CombineTags(tags []string) string {
	return strings.Join(tags, ", ")
}

// ExtractTags from a combined tags string.
func ExtractTags(tags string) []string {
	var list []string
	for _, one := range strings.Split(tags, ",") {
		if one = strings.TrimSpace(one); one != "" {
			list = append(list, one)
		}
	}
	return list
}

// AdjustedPoints returns the total points, taking levels and modifiers into account. 'entity' may be nil.
func AdjustedPoints(entity *Entity, canLevel bool, basePoints, levels, pointsPerLevel fxp.Int, cr selfctrl.Roll, modifiers []*TraitModifier, roundCostDown bool) fxp.Int {
	if !canLevel {
		levels = 0
		pointsPerLevel = 0
	}
	var baseEnh, levelEnh, baseLim, levelLim fxp.Int
	multiplier := cr.Multiplier()
	Traverse(func(mod *TraitModifier) bool {
		modifier := mod.CostModifier()
		switch mod.CostType {
		case tmcost.Percentage:
			switch mod.Affects {
			case affects.Total:
				if modifier < 0 {
					baseLim += modifier
					levelLim += modifier
				} else {
					baseEnh += modifier
					levelEnh += modifier
				}
			case affects.BaseOnly:
				if modifier < 0 {
					baseLim += modifier
				} else {
					baseEnh += modifier
				}
			case affects.LevelsOnly:
				if modifier < 0 {
					levelLim += modifier
				} else {
					levelEnh += modifier
				}
			}
		case tmcost.Points:
			if mod.Affects == affects.LevelsOnly {
				if canLevel {
					pointsPerLevel += modifier
				}
			} else {
				basePoints += modifier
			}
		case tmcost.Multiplier:
			multiplier = multiplier.Mul(modifier)
		}
		return false
	}, true, true, modifiers...)
	modifiedBasePoints := basePoints
	leveledPoints := pointsPerLevel.Mul(levels)
	if baseEnh != 0 || baseLim != 0 || levelEnh != 0 || levelLim != 0 {
		if SheetSettingsFor(entity).UseMultiplicativeModifiers {
			if baseEnh == levelEnh && baseLim == levelLim {
				modifiedBasePoints = modifyPoints(modifyPoints(modifiedBasePoints+leveledPoints, baseEnh), (-fxp.Eighty).Max(baseLim))
			} else {
				modifiedBasePoints = modifyPoints(modifyPoints(modifiedBasePoints, baseEnh), (-fxp.Eighty).Max(baseLim)) +
					modifyPoints(modifyPoints(leveledPoints, levelEnh), (-fxp.Eighty).Max(levelLim))
			}
		} else {
			baseMod := (-fxp.Eighty).Max(baseEnh + baseLim)
			levelMod := (-fxp.Eighty).Max(levelEnh + levelLim)
			if baseMod == levelMod {
				modifiedBasePoints = modifyPoints(modifiedBasePoints+leveledPoints, baseMod)
			} else {
				modifiedBasePoints = modifyPoints(modifiedBasePoints, baseMod) + modifyPoints(leveledPoints, levelMod)
			}
		}
	} else {
		modifiedBasePoints += leveledPoints
	}
	return fxp.ApplyRounding(modifiedBasePoints.Mul(multiplier), roundCostDown)
}

func modifyPoints(points, modifier fxp.Int) fxp.Int {
	return points + calculateModifierPoints(points, modifier)
}

func calculateModifierPoints(points, modifier fxp.Int) fxp.Int {
	return points.Mul(modifier).Div(fxp.Hundred)
}

// Kind returns the kind of data.
func (t *Trait) Kind() string {
	return t.kind(i18n.Text("Trait"))
}

// ClearUnusedFieldsForType zeroes out the fields that are not applicable to this type (container vs not-container).
func (t *Trait) ClearUnusedFieldsForType() {
	t.clearUnusedFields()
	if t.Container() {
		t.BasePoints = 0
		t.Levels = 0
		t.PointsPerLevel = 0
		t.CanLevel = false
		t.Prereq = nil
		t.Weapons = nil
		t.Features = nil
		t.RoundCostDown = false
		t.StudyHoursNeeded = study.Standard
		if t.TemplatePicker == nil {
			t.TemplatePicker = &TemplatePicker{}
		}
	} else {
		t.ContainerType = 0
		t.TemplatePicker = nil
		t.Ancestry = ""
		if !t.CanLevel {
			t.Levels = 0
			t.PointsPerLevel = 0
		}
	}
}

// CopyFrom implements node.EditorData.
func (t *Trait) CopyFrom(other *Trait) {
	t.copyFrom(other.Entity, other, false)
	t.LocalID = tid.MustNewTID(t.LocalID[0])
}

// ApplyTo implements node.EditorData.
func (t *Trait) ApplyTo(other *Trait) {
	id := other.LocalID
	other.copyFrom(other.Entity, t, true)
	other.LocalID = id
}

func (t *Trait) copyFrom(entity *Entity, other *Trait, isApply bool) {
	t.TraitData = other.TraitData
	t.Tags = txt.CloneStringSlice(t.Tags)
	t.Modifiers = nil
	if len(other.Modifiers) != 0 {
		t.Modifiers = make([]*TraitModifier, 0, len(other.Modifiers))
		for _, one := range other.Modifiers {
			t.Modifiers = append(t.Modifiers, one.Clone(entity, nil, true))
		}
	}
	t.Prereq = t.Prereq.CloneResolvingEmpty(t.Container(), isApply)
	t.Weapons = nil
	if len(other.Weapons) != 0 {
		t.Weapons = make([]*Weapon, len(other.Weapons))
		for i := range other.Weapons {
			t.Weapons[i] = other.Weapons[i].Clone(entity, nil, true)
		}
	}
	t.Features = other.Features.Clone()
	if len(other.Study) != 0 {
		t.Study = make([]*Study, len(other.Study))
		for i := range other.Study {
			t.Study[i] = other.Study[i].Clone()
		}
	}
	t.TemplatePicker = t.TemplatePicker.Clone()
}
