// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuctesting

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/juju/names/v5"
	"github.com/juju/testing"

	"github.com/juju/juju/internal/worker/uniter/runner/jujuc"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/storage"
)

// Storage holds the values for the hook context.
type Storage struct {
	Storage    map[names.StorageTag]jujuc.ContextStorageAttachment
	StorageTag names.StorageTag
	Added      map[string]params.StorageConstraints
}

// SetAttachment adds the attachment to the storage.
func (s *Storage) SetAttachment(attach jujuc.ContextStorageAttachment) {
	if attach == nil || attach == jujuc.ContextStorageAttachment(nil) {
		return
	}
	if s.Storage == nil {
		s.Storage = make(map[names.StorageTag]jujuc.ContextStorageAttachment)
	}
	s.Storage[attach.Tag()] = attach
}

// SetNewAttachment adds the attachment to the storage.
func (s *Storage) SetNewAttachment(name, location string, kind storage.StorageKind, stub *testing.Stub) {
	tag := names.NewStorageTag(name)
	attachment := &ContextStorageAttachment{
		info: &StorageAttachment{tag, kind, location},
	}
	attachment.stub = stub
	s.SetAttachment(attachment)
}

// SetBlockStorage adds the attachment to the storage.
func (s *Storage) SetBlockStorage(name, location string, stub *testing.Stub) {
	s.SetNewAttachment(name, location, storage.StorageKindBlock, stub)
}

// SetStorageTag sets the storage tag to the given ID.
func (s *Storage) SetStorageTag(id string) {
	tag := names.NewStorageTag(id)
	if _, ok := s.Storage[tag]; !ok {
		panic(fmt.Sprintf("storage %q not added yet", id))
	}
	s.StorageTag = tag
}

// SetUnitStorage sets storage that should be added.
func (s *Storage) SetUnitStorage(name string, constraints params.StorageConstraints) {
	if s.Added == nil {
		s.Added = make(map[string]params.StorageConstraints)
	}
	s.Added[name] = constraints
}

// AddUnitStorage sets storage that should be added.
func (s *Storage) AddUnitStorage(all map[string]params.StorageConstraints) {
	if s.Added == nil {
		s.Added = make(map[string]params.StorageConstraints)
	}
	for k, v := range all {
		s.Added[k] = v
	}
}

// ContextStorage is a test double for jujuc.ContextStorage.
type ContextStorage struct {
	contextBase
	info *Storage
}

// StorageTags implements jujuc.ContextStorage.
func (c *ContextStorage) StorageTags() ([]names.StorageTag, error) {
	c.stub.AddCall("StorageTags")

	tags := names.NewSet()
	for tag := range c.info.Storage {
		tags.Add(tag)
	}
	storageTags := make([]names.StorageTag, tags.Size())
	for i, tag := range tags.SortedValues() {
		storageTags[i] = tag.(names.StorageTag)
	}
	return storageTags, c.stub.NextErr()
}

// Storage implements jujuc.ContextStorage.
func (c *ContextStorage) Storage(tag names.StorageTag) (jujuc.ContextStorageAttachment, error) {
	c.stub.AddCall("Storage")

	storage, ok := c.info.Storage[tag]
	var err error
	if !ok {
		err = errors.NotFoundf("storage")
	}
	return storage, err

}

// HookStorage implements jujuc.ContextStorage.
func (c *ContextStorage) HookStorage() (jujuc.ContextStorageAttachment, error) {
	c.stub.AddCall("HookStorage")

	return c.Storage(c.info.StorageTag)
}

// AddUnitStorage implements jujuc.ContextStorage.
func (c *ContextStorage) AddUnitStorage(all map[string]params.StorageConstraints) error {
	c.stub.AddCall("AddUnitStorage", all)
	c.info.AddUnitStorage(all)
	return c.stub.NextErr()
}
