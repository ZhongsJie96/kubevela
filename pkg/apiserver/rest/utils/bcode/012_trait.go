package bcode

var ErrStorageTraitNotExists = NewBcode(400, 12000, "storage trait is not exists")

var ErrStorageDataNotExists = NewBcode(400, 12001, "storage trait data is not exists")

var ErrStorageMountPathNotExists = NewBcode(400, 12002, "storage trait mount path is not exists")

var ErrStorageTraitTypeNotExists = NewBcode(400, 12003, "storage trait type is not exists")

var ErrStorageTraitTypeNotSupport = NewBcode(400, 12004, "storage trait type not support")

var ErrStorageTraitKeyIsExists = NewBcode(400, 12005, "storage trait data is exists")

var ErrTypeAssert = NewBcode(400, 12006, "type assert err")
