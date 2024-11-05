// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package arvados

import (
	"time"
)

// Group is an arvados#group record
type Group struct {
	UUID               string                 `json:"uuid"`
	Name               string                 `json:"name"`
	OwnerUUID          string                 `json:"owner_uuid"`
	GroupClass         string                 `json:"group_class"`
	Etag               string                 `json:"etag"`
	TrashAt            *time.Time             `json:"trash_at"`
	CreatedAt          time.Time              `json:"created_at"`
	ModifiedAt         time.Time              `json:"modified_at"`
	ModifiedByUserUUID string                 `json:"modified_by_user_uuid"`
	DeleteAt           *time.Time             `json:"delete_at"`
	IsTrashed          bool                   `json:"is_trashed"`
	Properties         map[string]interface{} `json:"properties"`
	WritableBy         []string               `json:"writable_by,omitempty"`
	Description        string                 `json:"description"`
	FrozenByUUID       string                 `json:"frozen_by_uuid"`
	CanWrite           bool                   `json:"can_write"`
	CanManage          bool                   `json:"can_manage"`
}

// GroupList is an arvados#groupList resource.
type GroupList struct {
	Items          []Group       `json:"items"`
	ItemsAvailable int           `json:"items_available"`
	Offset         int           `json:"offset"`
	Limit          int           `json:"limit"`
	Included       []interface{} `json:"included"`
}

// ObjectList is an arvados#objectList resource.
type ObjectList struct {
	Included       []interface{} `json:"included"`
	Items          []interface{} `json:"items"`
	ItemsAvailable int           `json:"items_available"`
	Offset         int           `json:"offset"`
	Limit          int           `json:"limit"`
}

func (g Group) resourceName() string {
	return "group"
}
