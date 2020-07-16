package models

import (
	"errors"
	"time"
	"strings"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Field contains the fields needed for a user -> field mapping
// Fields contain 1..* Values
type Field struct {
	Id           int64     		`json:"id"`
	UserId       int64     		`json:"-"`
	Name         string    		`json:"name"`
	ModifiedDate time.Time 		`json:"modified_date"`
	FieldValues  []FieldValue   `json:"values" sql:"-"`
}

// FieldSummaries is a struct representing the overview of Fields.
type FieldSummaries struct {
	Total  int64          `json:"total"`
	Fields []FieldSummary `json:"fields"`
}

// FieldSummary represents a summary of the Field model. The only
// difference is that, instead of listing the Values (which could be expensive
// for large fields), it lists the value count.
type FieldSummary struct {
	Id           int64     `json:"id"`
	Name         string    `json:"name"`
	ModifiedDate time.Time `json:"modified_date"`
	NumValues    int64     `json:"num_values"`
}

// TargetFields is used for a many-to-many relationship between 1..* Fields and 1..* Values
type TargetFields struct {
	Id		 int64		`json:"-"`
	FieldId  int64 		`json:"-"`
	TargetId int64 		`json:"-"`
	Value    string		`json:"value"`
}

// FieldValue is used for a many-to-many relationship between 1..* Fields and 1..* Values
type FieldValue struct {
	FieldId  int64 		`json:"-"`
	TargetId int64 		`json:"target_id"`
	Email 	 string		`json:"email"`
	Value    string		`json:"value"`
}

// IdStruct is used a basic Id value
type IdStruct struct {
	Id  	 int64 		`json:"-"`
}

// ErrFieldNameNotSpecified is thrown when a field name is not specified
var ErrFieldNameNotSpecified = errors.New("Field name not specified")

// ErrNoValuesSpecified is thrown when no values are specified by the user
var ErrNoValuesSpecified = errors.New("No values specified")

// Validate performs validation on a field given by the user
func (f *Field) Validate() error {
	switch {
	case f.Name == "":
		return ErrFieldNameNotSpecified
	case len(f.FieldValues) == 0:
		return ErrNoValuesSpecified
	}
	f.Name = strings.ToLower(f.Name)
	return nil
}

// GetFields returns the fields owned by the given user.
func GetFields(uid int64) ([]Field, error) {
	fs := []Field{}
	err := db.Where("user_id=?", uid).Find(&fs).Error
	if err != nil {
		log.Error(err)
		return fs, err
	}
	for i := range fs {
		fs[i].FieldValues, err = GetFieldValues(fs[i].Id)
		if err != nil {
			log.Error(err)
		}
	}
	return fs, nil
}

// GetFieldSummaries returns the summaries for the fields
// created by the given uid.
func GetFieldSummaries(uid int64) (FieldSummaries, error) {
	fs := FieldSummaries{}
	query := db.Table("fields").Where("user_id=?", uid)
	err := query.Select("id, name, modified_date").Scan(&fs.Fields).Error
	if err != nil {
		log.Error(err)
		return fs, err
	}
	for i := range fs.Fields {
		query = db.Table("target_fields").Where("field_id=?", fs.Fields[i].Id)
		err = query.Count(&fs.Fields[i].NumValues).Error
		if err != nil {
			return fs, err
		}
	}
	fs.Total = int64(len(fs.Fields))
	return fs, nil
}

// GetField returns the field, if it exists, specified by the given id and user_id.
func GetField(id int64, uid int64) (Field, error) {
	f := Field{}
	err := db.Where("user_id=? and id=?", uid, id).Find(&f).Error
	if err != nil {
		log.Error(err)
		return f, err
	}
	f.FieldValues, err = GetFieldValues(f.Id)
	if err != nil {
		log.Error(err)
	}
	return f, nil
}

// GetFieldSummary returns the summary for the requested field
func GetFieldSummary(id int64, uid int64) (FieldSummary, error) {
	f := FieldSummary{}
	query := db.Table("fields").Where("user_id=? and id=?", uid, id)
	err := query.Select("id, name, modified_date").Scan(&f).Error
	if err != nil {
		log.Error(err)
		return f, err
	}
	query = db.Table("target_fields").Where("field_id=?", id)
	err = query.Count(&f.NumValues).Error
	if err != nil {
		return f, err
	}
	return f, nil
}

// GetFieldByName returns the field, if it exists, specified by the given name and user_id.
func GetFieldByName(n string, uid int64) (Field, error) {
	f := Field{}
	err := db.Where("user_id=? and name=?", uid, n).Find(&f).Error
	if err != nil {
		log.Error(err)
		return f, err
	}
	f.FieldValues, err = GetFieldValues(f.Id)
	if err != nil {
		log.Error(err)
	}
	return f, err
}

// PostField creates a new field in the database.
func PostField(f *Field) error {
	if err := f.Validate(); err != nil {
		return err
	}
	// Insert the field into the DB
	tx := db.Begin()
	err := tx.Save(f).Error
	if err != nil {
		tx.Rollback()
		log.Error(err)
		return err
	}
	for _, t := range f.FieldValues {
		new_target_id := IdStruct{}
		err = tx.Table("targets").Select("id").Where("email=?", strings.ToLower(t.Email)).First(&new_target_id).Error
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
		// If the target exists
		if new_target_id.Id != 0 {
			dbv := TargetFields{}
			dbv.TargetId = new_target_id.Id
			dbv.FieldId = f.Id
			dbv.Value = t.Value
			err = insertValueIntoField(tx, dbv, f.Id)
			if err != nil {
				log.Error(err)
				tx.Rollback()
				return err
			}
		} 
	}
	err = tx.Commit().Error
	if err != nil {
		log.Error(err)
		tx.Rollback()
		return err
	}
	return nil
}

// PutField updates the given field if found in the database.
func PutField(f *Field) error {
	if err := f.Validate(); err != nil {
		return err
	}
	// Fetch field's existing values from database.
	vs := []FieldValue{}
	vs, err := GetFieldValues(f.Id)
	if err != nil {
		log.WithFields(logrus.Fields{
			"field_id": f.Id,
		}).Error("Error getting values from field")
		return err
	}
	
	// Preload the caches
	cacheNew := make(map[string]int64, len(f.FieldValues))
	for _, v := range f.FieldValues {
		cacheNew[v.Email] = v.TargetId
	}
	
	cacheExisting := make(map[string]int64, len(vs))
	for _, v := range vs {
		cacheExisting[v.Email] = v.TargetId
	}
	
	tx := db.Begin()
	// Check existing values, removing any that are no longer in the field.
	for _, v := range vs {
		if _, ok := cacheNew[v.Email]; ok {
			continue
		}
		// If the value does not exist in the field any longer, we delete it
		err := tx.Where("field_id=? and target_id=?", f.Id, v.TargetId).Delete(&TargetFields{}).Error
		if err != nil {
			tx.Rollback()
			log.WithFields(logrus.Fields{
				"email": v.Email,
			}).Error("Error deleting email")
		}
	}
	// Add any values that are not in the database yet.
	for _, nv := range f.FieldValues {
		// If the value already exists in the database, we don't do anything
		// the record with the latest information.
		if _, ok := cacheExisting[nv.Email]; ok {
			continue
		}
		// Otherwise, add value if not in database
		new_target_id := IdStruct{}
		err = tx.Table("targets").Select("id").Where("email=?", strings.ToLower(nv.Email)).First(&new_target_id).Error
		if err != nil {
			log.Error(err)
			tx.Rollback()
			return err
		}
		
		// If the target exists
		if new_target_id.Id != 0 {
			dbv := TargetFields{}
			dbv.TargetId = new_target_id.Id
			dbv.FieldId = f.Id
			dbv.Value = nv.Value
			err = insertValueIntoField(tx, dbv, f.Id)
			if err != nil {
				log.Error(err)
				tx.Rollback()
				return err
			}
		}
	}
	err = tx.Save(f).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// DeleteField deletes a given field by field ID and user ID
func DeleteField(f *Field) error {
	// Delete all the target_fields entries for this field
	err := db.Where("field_id=?", f.Id).Delete(&TargetFields{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	// Delete the field itself
	err = db.Delete(f).Error
	if err != nil {
		log.Error(err)
		return err
	}
	return err
}

func insertValueIntoField(vx *gorm.DB, v TargetFields, fid int64) error {
	err := vx.Save(&TargetFields{FieldId: fid, TargetId: v.TargetId, Value: v.Value}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetFieldValues performs a many-to-many select to get all the values for a field
func GetFieldValues(fid int64) ([]FieldValue, error) {
	vs := []FieldValue{}
	err := db.Table("target_fields").Select("field_id, target_id, value, email").Joins("left join targets t ON target_fields.target_id = t.id").Where("field_id=?", fid).Scan(&vs).Error
	return vs, err
}
