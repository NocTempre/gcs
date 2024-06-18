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

	"github.com/richardwilkes/gcs/v5/model/fxp"
	"github.com/richardwilkes/gcs/v5/model/gurps/enums/container"
	"github.com/richardwilkes/gcs/v5/model/jio"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/eval"
	xfs "github.com/richardwilkes/toolbox/xio/fs"
)

// DefaultAncestry holds the name of the default ancestry.
const (
	DefaultAncestry = "Human"
	ancestryTypeKey = "ancestry"
)

// Ancestry holds details necessary to generate ancestry-specific customizations.
type Ancestry struct {
	Name          string                     `json:"name,omitempty"`
	CommonOptions *AncestryOptions           `json:"common_options,omitempty"`
	GenderOptions []*WeightedAncestryOptions `json:"gender_options,omitempty"`
}

type ancestryData struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	Ancestry
}

// AvailableAncestries scans the libraries and returns the available ancestries.
func AvailableAncestries(libraries Libraries) []*NamedFileSet {
	return ScanForNamedFileSets(embeddedFS, "embedded_data", true, libraries, AncestryExt)
}

// LookupAncestry an Ancestry by name.
func LookupAncestry(name string, libraries Libraries) *Ancestry {
	for _, lib := range AvailableAncestries(libraries) {
		for _, one := range lib.List {
			if one.Name == name {
				if a, err := NewAncestryFromFile(one.FileSystem, one.FilePath); err != nil {
					errs.Log(err, "path", one.FilePath)
				} else {
					return a
				}
			}
		}
	}
	return nil
}

// NewAncestryFromFile creates a new Ancestry from a file.
func NewAncestryFromFile(fileSystem fs.FS, filePath string) (*Ancestry, error) {
	var ancestry ancestryData
	if err := jio.LoadFromFS(context.Background(), fileSystem, filePath, &ancestry); err != nil {
		return nil, err
	}
	if ancestry.Type == "" && ancestry.Version == 0 { // for some older files
		ancestry.Type = ancestryTypeKey
		ancestry.Version = CurrentDataVersion
	}
	if ancestry.Type != ancestryTypeKey {
		return nil, errs.New(UnexpectedFileDataMsg())
	}
	if err := CheckVersion(ancestry.Version); err != nil {
		return nil, err
	}
	if ancestry.Name == "" {
		ancestry.Name = xfs.BaseName(filePath)
	}
	return &ancestry.Ancestry, nil
}

// Save writes the Ancestry to the file as JSON.
func (a *Ancestry) Save(filePath string) error {
	return jio.SaveToFile(context.Background(), filePath, &ancestryData{
		Type:     ancestryTypeKey,
		Version:  CurrentDataVersion,
		Ancestry: *a,
	})
}

// RandomGender returns a randomized gender.
func (a *Ancestry) RandomGender(not string) string {
	if choice := ChooseWeightedAncestryOptions(a.GenderOptions, func(o *AncestryOptions) bool {
		return o.Name == not
	}); choice != nil {
		return choice.Name
	}
	return ""
}

// GenderedOptions returns the options for the specified gender, or nil.
func (a *Ancestry) GenderedOptions(gender string) *AncestryOptions {
	gender = strings.TrimSpace(gender)
	for _, one := range a.GenderOptions {
		if strings.EqualFold(one.Value.Name, gender) {
			return one.Value
		}
	}
	return nil
}

// RandomHeight returns a randomized height.
func (a *Ancestry) RandomHeight(resolver eval.VariableResolver, gender string, not fxp.Length) fxp.Length {
	if options := a.GenderedOptions(gender); options != nil && options.HeightFormula != "" {
		return options.RandomHeight(resolver, not)
	}
	if a.CommonOptions != nil && a.CommonOptions.HeightFormula != "" {
		return a.CommonOptions.RandomHeight(resolver, not)
	}
	return fxp.LengthFromInteger(defaultHeight, fxp.Inch)
}

// RandomWeight returns a randomized weight.
func (a *Ancestry) RandomWeight(resolver eval.VariableResolver, gender string, not fxp.Weight) fxp.Weight {
	if options := a.GenderedOptions(gender); options != nil && options.WeightFormula != "" {
		return options.RandomWeight(resolver, not)
	}
	if a.CommonOptions != nil && a.CommonOptions.WeightFormula != "" {
		return a.CommonOptions.RandomWeight(resolver, not)
	}
	return fxp.WeightFromInteger(defaultWeight, fxp.Pound)
}

// RandomAge returns a randomized age.
func (a *Ancestry) RandomAge(resolver eval.VariableResolver, gender string, not int) int {
	if options := a.GenderedOptions(gender); options != nil && options.AgeFormula != "" {
		return options.RandomAge(resolver, not)
	}
	if a.CommonOptions != nil && a.CommonOptions.AgeFormula != "" {
		return a.CommonOptions.RandomAge(resolver, not)
	}
	return defaultAge
}

// RandomHair returns a randomized hair.
func (a *Ancestry) RandomHair(gender, not string) string {
	if options := a.GenderedOptions(gender); options != nil && len(options.HairOptions) != 0 {
		return options.RandomHair(not)
	}
	if a.CommonOptions != nil && len(a.CommonOptions.HairOptions) != 0 {
		return a.CommonOptions.RandomHair(not)
	}
	return defaultHair
}

// RandomEyes returns a randomized eyes.
func (a *Ancestry) RandomEyes(gender, not string) string {
	if options := a.GenderedOptions(gender); options != nil && len(options.EyeOptions) != 0 {
		return options.RandomEye(not)
	}
	if a.CommonOptions != nil && len(a.CommonOptions.EyeOptions) != 0 {
		return a.CommonOptions.RandomEye(not)
	}
	return defaultEye
}

// RandomSkin returns a randomized skin.
func (a *Ancestry) RandomSkin(gender, not string) string {
	if options := a.GenderedOptions(gender); options != nil && len(options.SkinOptions) != 0 {
		return options.RandomSkin(not)
	}
	if a.CommonOptions != nil && len(a.CommonOptions.SkinOptions) != 0 {
		return a.CommonOptions.RandomSkin(not)
	}
	return defaultSkin
}

// RandomHandedness returns a randomized handedness.
func (a *Ancestry) RandomHandedness(gender, not string) string {
	if options := a.GenderedOptions(gender); options != nil && len(options.HandednessOptions) != 0 {
		return options.RandomHandedness(not)
	}
	if a.CommonOptions != nil && len(a.CommonOptions.HandednessOptions) != 0 {
		return a.CommonOptions.RandomHandedness(not)
	}
	return defaultHandedness
}

// RandomName returns a randomized name.
func (a *Ancestry) RandomName(nameGeneratorRefs []*NameGeneratorRef, gender string) string {
	if options := a.GenderedOptions(gender); options != nil && len(options.NameGenerators) != 0 {
		return options.RandomName(nameGeneratorRefs)
	}
	if a.CommonOptions != nil && len(a.CommonOptions.NameGenerators) != 0 {
		return a.CommonOptions.RandomName(nameGeneratorRefs)
	}
	return ""
}

// ActiveAncestries returns a list of Ancestry nodes that are enabled in the given Trait nodes and their descendants.
func ActiveAncestries(list []*Trait) []*Ancestry {
	var ancestries []*Ancestry
	libraries := GlobalSettings().Libraries()
	Traverse(func(t *Trait) bool {
		if t.Container() && t.ContainerType == container.Ancestry && t.Enabled() {
			if anc := LookupAncestry(t.Ancestry, libraries); anc != nil {
				ancestries = append(ancestries, anc)
			}
		}
		return false
	}, true, false, list...)
	return ancestries
}

// ActiveAncestryTraits returns the Traits that have Ancestry data and are enabled within the given traits or their
// descendants.
func ActiveAncestryTraits(list []*Trait) []*Trait {
	var result []*Trait
	libraries := GlobalSettings().Libraries()
	Traverse(func(t *Trait) bool {
		if t.Container() && t.ContainerType == container.Ancestry && t.Enabled() {
			if ancestry := LookupAncestry(t.Ancestry, libraries); ancestry != nil {
				result = append(result, t)
			}
		}
		return false
	}, true, false, list...)
	return result
}
