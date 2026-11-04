package database

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// RecordID is a SurrealDB record id serialized as "<table>:<id>", e.g. "users:50cqyhrtc8piv0b6lf51".
//
// SurrealDB returns record ids as CBOR tags (models.RecordID), which the driver
// can only decode into a models.RecordID or interface{}, never a plain string.
// A model field of type string therefore stays empty after a query. This type
// decodes the tag into the canonical "table:id" string form so model primary
// keys are populated on reads while still behaving as a plain string elsewhere.
type RecordID string

func (id *RecordID) UnmarshalCBOR(data []byte) error {
	var rid models.RecordID
	if err := cbor.Unmarshal(data, &rid); err == nil {
		*id = RecordID(rid.String())
		return nil
	}

	var s string
	if err := cbor.Unmarshal(data, &s); err == nil {
		*id = RecordID(s)
	}
	return nil
}

// ParseRecordID parses a "<table>:<id>" string into a surrealdb RecordID value
// for use as the "what" argument of Select/Merge/Delete/Update. Passing a plain
// string like "users:abc" to those methods treats it as a *table name* in
// surrealdb.go v1.6, so record ids must be passed as a models.RecordID.
func ParseRecordID(id string) (models.RecordID, error) {
	parsed, err := models.ParseRecordID(id)
	if err != nil {
		return models.RecordID{}, err
	}
	return *parsed, nil
}
