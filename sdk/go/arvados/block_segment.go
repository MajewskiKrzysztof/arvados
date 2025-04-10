// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package arvados

import (
	"encoding/json"
	"fmt"
)

// BlockSegment is a portion of a block stored in Keep. It is used in
// the replace_segments API.
type BlockSegment struct {
	Locator string
	Offset  int
	Length  int
}

func (bs *BlockSegment) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	_, err := fmt.Sscanf(s, "%s %d %d", &bs.Locator, &bs.Offset, &bs.Length)
	return err
}

// MarshalText enables encoding/json to encode BlockSegment as a map
// key.
func (bs BlockSegment) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%s %d %d", bs.Locator, bs.Offset, bs.Length)), nil
}

// MarshalText enables encoding/json to decode BlockSegment as a map
// key.
func (bs *BlockSegment) UnmarshalText(p []byte) error {
	_, err := fmt.Sscanf(string(p), "%s %d %d", &bs.Locator, &bs.Offset, &bs.Length)
	return err
}

func (bs BlockSegment) StripAllHints() BlockSegment {
	bs.Locator = stripAllHints(bs.Locator)
	return bs
}
